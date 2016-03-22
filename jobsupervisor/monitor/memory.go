// +build windows

package monitor

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa366589(v=vs.85).aspx
	procGlobalMemoryStatusEx = kernel32DLL.MustFindProc("GlobalMemoryStatusEx")
)

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
