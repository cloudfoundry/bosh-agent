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

var (
	// Global kernel32 DLL
	kernel32DLL = syscall.NewLazyDLL("kernel32")

	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724400(v=vs.85).aspx
	procGetSystemTimes = kernel32DLL.NewProc("GetSystemTimes")
)

func init() {
	if err := procGetSystemTimes.Find(); err != nil {
		panic(fmt.Errorf("monitor: %s", err))
	}
}

type cpuTime struct {
	previous uint64
	load     float64
}

type Monitor struct {
	user   cpuTime
	kernel cpuTime
	idle   cpuTime
	mu     sync.RWMutex
	err    error
	freq   time.Duration
	inited bool //
}

func New(freq time.Duration) (*Monitor, error) {
	if freq < time.Millisecond*15 {
		freq = time.Millisecond * 100
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

type CPU struct {
	User   float64
	Kernel float64
	Idle   float64
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

func (m *Monitor) updateCPULoad() (err error) {
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
	if r1 == 0 {
		if e1 != 0 {
			err = fmt.Errorf("GetSystemTimes: %s", error(e1))
		} else {
			err = fmt.Errorf("GetSystemTimes: %s", syscall.EINVAL)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	kernelTicks := kernelTime.Uint64()
	userTicks := userTime.Uint64()
	idleTicks := idleTime.Uint64()

	totalTicks := kernelTicks + userTicks
	total := totalTicks - (m.kernel.previous + m.user.previous)
	if total <= 0 {
		return nil
	}

	idle := idleTicks - m.idle.previous
	load := 1 - (float64(idle) / float64(total))

	m.idle.load = 1 - load
	m.kernel.load = load * math.Abs(1-(float64(kernelTicks)/float64(totalTicks)))
	m.user.load = load * math.Abs(1-(float64(userTicks)/float64(totalTicks)))

	m.kernel.previous = kernelTicks
	m.user.previous = userTicks
	m.idle.previous = idleTicks

	return nil
}
