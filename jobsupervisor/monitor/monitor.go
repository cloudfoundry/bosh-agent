// +build windows

package monitor

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var Default *Monitor

var (
	// Global kernel32 DLL
	kernel32DLL = syscall.NewLazyDLL("kernel32")

	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724400(v=vs.85).aspx
	procGetSystemTimes = kernel32DLL.NewProc("GetSystemTimes")

	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms683223(v=vs.85).aspx
	procGetProcessTimes = kernel32DLL.NewProc("GetProcessTimes")
)

func init() {
	var err error
	if err = procGetSystemTimes.Find(); err != nil {
		panic(fmt.Errorf("monitor: %s", err))
	}

	Default, err = New(-1)
	if err != nil {
		panic(fmt.Errorf("monitor: initializing Default: %s", err))
	}
}

type CPU struct {
	User   float64
	Kernel float64
	Idle   float64
}

// Total returns the sum of user and kernel CPU time.
func (c CPU) Total() float64 {
	return c.User + c.Kernel
}

type CPUTime struct {
	previous uint64
	delta    uint64
	load     float64
}

func (c CPUTime) CPU() float64 { return c.load }

type Process struct {
	pid    uint32
	name   string
	cpu    CPUTime
	mem    MemStat
	handle windows.Handle
}

func (p *Process) Pid() uint32  { return p.pid }
func (p *Process) Name() string { return p.name }
func (p *Process) CPU() CPUTime { return p.cpu }
func (p *Process) Mem() MemStat { return p.mem }

type Monitor struct {
	user   CPUTime
	kernel CPUTime
	idle   CPUTime
	mu     sync.RWMutex
	err    error
	freq   time.Duration
	inited bool                // monitor initialized
	pids   map[uint32]*Process // pid => Process name
}

func New(freq time.Duration) (*Monitor, error) {
	if freq < time.Millisecond*10 {
		freq = time.Second
	}
	m := &Monitor{
		freq:   freq,
		inited: true,
	}
	if err := m.monitorCPU(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Monitor) WatchProcess(name string, pid uint32) error {
	if _, ok := m.getProc(pid); ok {
		return nil
	}

	handle, err := windows.OpenProcess(process_query_information, false, pid)
	if err != nil {
		return fmt.Errorf("opening handle for process (%d): %s", pid, err)
	}

	p := &Process{
		name:   name,
		pid:    pid,
		handle: handle,
	}
	m.setProc(p)
	return nil
}

func (m *Monitor) LookupProcess(pid uint32) (*Process, bool) {
	return m.getProc(pid)
}

func (m *Monitor) CPU() (cpu CPU, err error) {
	m.mu.RLock()
	if !m.inited {
		err = errors.New("monitor: not initialized")
	}
	if m.err != nil {
		err = m.err
	}
	cpu = CPU{
		Kernel: m.kernel.load,
		User:   m.user.load,
		Idle:   m.idle.load,
	}
	m.mu.RUnlock()
	return
}

func (m *Monitor) getProc(pid uint32) (p *Process, ok bool) {
	m.mu.RLock()
	if m.pids != nil {
		p, ok = m.pids[pid]
	}
	m.mu.RUnlock()
	return
}

func (m *Monitor) setProc(p *Process) {
	m.mu.Lock()
	if m.pids == nil {
		m.pids = make(map[uint32]*Process)
	}
	if p != nil {
		m.pids[p.pid] = p
	}
	m.mu.Unlock()
}

func (m *Monitor) monitorCPU() error {
	if err := m.updateSystemCPU(); err != nil {
		m.err = err
		return m.err
	}
	go func() {
		tick := time.NewTicker(m.freq)
		for _ = range tick.C {
			//Hard error
			if err := m.updateSystemCPU(); err != nil {
				m.err = err
				return
			}
			if err := m.updateProcessesCPU(); err != nil {
				// Soft error
			}
		}
	}()
	return nil
}

func (m *Monitor) updateSystemCPU() error {
	if m.err != nil {
		return m.err
	}
	var (
		idleTime   filetime
		kernelTime filetime
		userTime   filetime
	)
	r1, _, e1 := syscall.Syscall(procGetSystemTimes.Addr(), 3,
		uintptr(unsafe.Pointer(&idleTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if err := checkErrno(r1, e1); err != nil {
		m.err = fmt.Errorf("GetSystemTimes: %s", error(e1))
		return m.err
	}

	m.calculateSystemCPU(kernelTime.Uint64(), userTime.Uint64(), idleTime.Uint64())

	return nil
}

func (m *Monitor) calculateSystemCPU(kernelTicks, userTicks, idleTicks uint64) {
	m.mu.Lock()

	kernel := kernelTicks - m.kernel.previous
	user := userTicks - m.user.previous
	idle := idleTicks - m.idle.previous

	total := kernel + user
	if total > 0 {
		m.idle.load = float64(idle) / float64(total)
		m.idle.previous = idleTicks
		m.idle.delta = idle

		m.kernel.load = math.Max(float64(kernel-idle)/float64(total), 0)
		m.kernel.previous = kernelTicks
		m.kernel.delta = kernel

		m.user.load = math.Max(1-m.idle.load-m.kernel.load, 0)
		m.user.previous = userTicks
		m.user.delta = user
	} else {
		m.idle.load = 0
		m.kernel.load = 0
		m.user.load = 0
	}

	m.mu.Unlock()
}

func (m *Monitor) updateProcessesCPU() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var first error
	for _, p := range m.pids {
		var (
			creationTime filetime
			exitTime     filetime
			kernelTime   filetime
			userTime     filetime
		)
		r1, _, e1 := procGetProcessTimes.Call(
			uintptr(p.handle),
			uintptr(unsafe.Pointer(&creationTime)),
			uintptr(unsafe.Pointer(&exitTime)),
			uintptr(unsafe.Pointer(&kernelTime)),
			uintptr(unsafe.Pointer(&userTime)),
		)
		if err := checkErrno(r1, e1); err != nil {
			// The process likely died.
			if first == nil {
				first = err
			}
			defer windows.CloseHandle(p.handle)
			delete(m.pids, p.pid)
		}
		m.calculateProcessCPU(p, kernelTime.Uint64()+userTime.Uint64())
	}

	return first
}

func (m *Monitor) calculateProcessCPU(p *Process, ticks uint64) {

	p.cpu.delta = ticks - p.cpu.previous
	p.cpu.previous = ticks

	total := m.kernel.delta + m.user.delta - m.idle.delta
	if total > 0 {
		frac := float64(p.cpu.delta) / float64(total)
		load := m.kernel.load + m.user.load
		p.cpu.load = math.Min(frac*load, load)
	}
}

func checkErrno(r1 uintptr, err error) error {
	if r1 == 0 {
		if e, ok := err.(syscall.Errno); ok && e != 0 {
			return err
		}
		return syscall.EINVAL
	}
	return nil
}

// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724284(v=vs.85).aspx
type filetime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

func (f filetime) Uint64() uint64 {
	return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime)
}
