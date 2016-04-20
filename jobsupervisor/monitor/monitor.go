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

	"github.com/cloudfoundry/bosh-utils/state"
)

var (
	// Global kernel32 DLL
	kernel32DLL = syscall.MustLoadDLL("kernel32")

	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724400(v=vs.85).aspx
	procGetSystemTimes = kernel32DLL.MustFindProc("GetSystemTimes")
)

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

type Monitor struct {
	user   CPUTime
	kernel CPUTime
	idle   CPUTime
	mem    MemStat      // system memory
	tick   *time.Ticker // use tick.Stop() to stop monitoring
	err    error        // system error, if any
	inited bool         // monitor initialized
	mu     sync.RWMutex // pids mutex
	state  *state.State
	cond   *sync.Cond // Optional sync conditional for StatsCollector
}

func New(freq time.Duration) (*Monitor, error) {
	if freq < time.Millisecond*10 {
		freq = time.Millisecond * 500
	}
	st, err := state.NewState(state.Stopped, state.Running, state.Exited)
	if err != nil {
		return nil, err
	}
	m := &Monitor{
		tick:   time.NewTicker(freq),
		inited: true,
		state:  st,
	}
	if err := m.monitorLoop(); err != nil {
		return nil, err
	}
	return m, nil
}

// condMonitor, returns a Monitor that broadcasts on cond on each update.
func condMonitor(freq time.Duration, cond *sync.Cond) (*Monitor, error) {
	st, err := state.NewState(state.Stopped, state.Running, state.Exited)
	if err != nil {
		return nil, err
	}
	m := &Monitor{
		tick:   time.NewTicker(freq),
		inited: true,
		state:  st,
		cond:   cond,
	}
	m.state.Set(state.Running)
	if err := m.monitorLoop(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Monitor) MemStat() MemStat {
	m.mu.RLock()
	mem := m.mem
	m.mu.RUnlock()
	return mem
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

func (m *Monitor) monitorLoop() error {
	if err := m.updateSystemCPU(); err != nil {
		m.err = err
		return m.err
	}
	go func() {
		defer m.state.Set(state.Exited)
		for {
			select {
			case <-m.tick.C:
				if !m.state.Is(state.Running) {
					continue
				}
				if m.cond != nil {
					m.cond.Broadcast()
				}
				// Hard error
				if err := m.updateSystemCPU(); err != nil {
					m.err = err
					return
				}
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
