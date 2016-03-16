// +build windows

package monitor

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa366589(v=vs.85).aspx
	procGlobalMemoryStatusEx = kernel32DLL.NewProc("GlobalMemoryStatusEx")

	// TODO (CEV): Change other DLLs to use MustLoad
	psapiDLL = syscall.MustLoadDLL("psapi")

	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms683219(v=vs.85).aspx
	procGetProcessMemoryInfo = psapiDLL.MustFindProc("GetProcessMemoryInfo")
)

func init() {
	if err := procGlobalMemoryStatusEx.Find(); err != nil {
		panic(fmt.Errorf("monitor: %s", err))
	}
}

type Byte uint64

const (
	KB Byte = 1 << (10 * (iota + 1))
	MB
	GB
)

func (b Byte) Uint64() uint64 { return uint64(b) }

func (b Byte) String() string {
	switch {
	case b < KB:
		return fmt.Sprintf("%d", b)
	case b < MB:
		return fmt.Sprintf("%.1fK", float64(b)/float64(KB))
	case b < GB:
		return fmt.Sprintf("%.1fM", float64(b)/float64(MB))
	}
	return fmt.Sprintf("%.1fG", float64(b)/float64(GB))
}

type MemStat struct {
	Total Byte
	Avail Byte
}

func (m MemStat) Used() float64 {
	if m.Avail == 0 {
		if m.Total == 0 {
			return 0
		}
		return 1
	}
	return 1 - float64(m.Avail)/float64(m.Total)
}

func SystemMemStats() (MemStat, error) {
	type memstat struct {
		Length               uint32
		MemoryLoad           uint32
		TotalPhys            uint64
		AvailPhys            uint64
		TotalPageFile        uint64
		AvailPageFile        uint64
		TotalVirtual         uint64
		AvailVirtual         uint64
		AvailExtendedVirtual uint64
	}

	var m memstat
	m.Length = uint32(unsafe.Sizeof(m))
	r1, _, e1 := syscall.Syscall(procGlobalMemoryStatusEx.Addr(), 1, uintptr(unsafe.Pointer(&m)), 0, 0)
	if err := checkErrno(r1, e1); err != nil {
		return MemStat{}, fmt.Errorf("SystemMemStats: %s", err)
	}
	mem := MemStat{
		Total: Byte(m.TotalPhys),
		Avail: Byte(m.AvailPhys),
	}
	return mem, nil
}

// https://msdn.microsoft.com/en-us/library/windows/desktop/ms684874(v=vs.85).aspx
type process_memory_counters_ex struct {
	cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

//https://msdn.microsoft.com/en-us/library/windows/desktop/ms684320(v=vs.85).aspx
func ProcessMemStats(pid uint32) (Byte, error) {
	// PROCESS_QUERY_INFORMATION
	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms684880(v=vs.85).aspx
	const access uint32 = 0x0400

	h, err := windows.OpenProcess(access, false, uint32(pid))
	if err != nil {
		return 0, fmt.Errorf("ProcessMemStats: %s", err)
	}
	defer windows.CloseHandle(h)

	var m process_memory_counters_ex
	r1, _, err := procGetProcessMemoryInfo.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&m)),
		unsafe.Sizeof(m),
	)
	if err = checkErrno(r1, err); err != nil {
		return 0, fmt.Errorf("ProcessMemStats: %s", err)
	}
	return Byte(m.WorkingSetSize), nil
}
