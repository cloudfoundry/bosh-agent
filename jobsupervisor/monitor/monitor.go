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
)

var Default *Monitor

var (
	// Global kernel32 DLL
	kernel32DLL = syscall.NewLazyDLL("kernel32")

	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724400(v=vs.85).aspx
	procGetSystemTimes = kernel32DLL.NewProc("GetSystemTimes")
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

type cpuTime struct {
	previous uint64
	load     float64
}

type process struct {
	Pid    uint32
	Name   string
	ignore bool
}

type Monitor struct {
	user   cpuTime
	kernel cpuTime
	idle   cpuTime
	mu     sync.RWMutex
	err    error
	freq   time.Duration
	inited bool              // monitor initialized
	pids   map[uint32]string // pid => process name
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

func (m *Monitor) WatchProcess() {

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

func (m *Monitor) monitorCPU() error {
	if err := m.updateCPULoad(); err != nil {
		m.err = err
		return m.err
	}
	go func() {
		tick := time.NewTicker(m.freq)
		for _ = range tick.C {
			if err := m.updateCPULoad(); err != nil {
				m.err = err
				return
			}
		}
	}()
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

func (m *Monitor) updateCPULoad() error {
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

	m.update(kernelTime.Uint64(), userTime.Uint64(), idleTime.Uint64())

	// m.mu.Lock()
	// defer m.mu.Unlock()

	// kernelTicks := kernelTime.Uint64()
	// userTicks := userTime.Uint64()
	// idleTicks := idleTime.Uint64()

	// totalTicks := kernelTicks + userTicks
	// total := totalTicks - (m.kernel.previous + m.user.previous)
	// if total <= 0 {
	// 	return nil
	// }

	// idle := idleTicks - m.idle.previous
	// load := 1 - (float64(idle) / float64(total))

	// m.idle.load = 1 - load
	// m.kernel.load = load * math.Abs(1-(float64(kernelTicks)/float64(totalTicks)))
	// m.user.load = load * math.Abs(1-(float64(userTicks)/float64(totalTicks)))
	// m.kernel.update(load, kernelTicks, totalTicks)
	// m.user.update(load, userTicks, totalTicks)

	// m.kernel.previous = kernelTicks
	// m.user.previous = userTicks
	// m.idle.previous = idleTicks

	return nil
}

// 3125000
// 1562500

// 0.25 idle
// 0.625

func (m *Monitor) update(kernelTicks, userTicks, idleTicks uint64) {
	m.mu.Lock()

	kernel := kernelTicks - m.kernel.previous
	user := userTicks - m.user.previous
	idle := idleTicks - m.idle.previous

	total := kernel + user
	if total > 0 {
		m.idle.load = float64(idle) / float64(total)
		m.idle.previous = idleTicks

		m.kernel.load = math.Max(float64(kernel-idle)/float64(total), 0)
		m.kernel.previous = kernelTicks

		m.user.load = math.Max(1-m.idle.load-m.kernel.load, 0)
		m.user.previous = userTicks
	} else {
		m.idle.load = 0
		m.kernel.load = 0
		m.user.load = 0
	}

	m.mu.Unlock()
}

func (c *cpuTime) update(load float64, ticks, total uint64) {
	c.load = load * math.Abs(1-(float64(ticks)/float64(total)))
	c.previous = ticks
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
