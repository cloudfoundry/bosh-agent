// +build windows

package monitor

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa364935(v=vs.85).aspx
	procGetDiskFreeSpace = kernel32DLL.NewProc("GetDiskFreeSpaceW")
)

func init() {
	if err := procGetDiskFreeSpace.Find(); err != nil {
		panic(fmt.Errorf("monitor: %s", err))
	}
}

// UsedDiskSpace returns the percentage of used disk space for drive name.
func UsedDiskSpace(name string) (float64, error) {
	var (
		sectorsPerCluster     uint32
		bytesPerSector        uint32
		numberOfFreeClusters  uint32
		totalNumberOfClusters uint32
	)
	root, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return -1, fmt.Errorf("UsedDiskSpace (%s): %s", name, err)
	}
	r1, _, e1 := syscall.Syscall6(procGetDiskFreeSpace.Addr(), 5,
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&sectorsPerCluster)),
		uintptr(unsafe.Pointer(&bytesPerSector)),
		uintptr(unsafe.Pointer(&numberOfFreeClusters)),
		uintptr(unsafe.Pointer(&totalNumberOfClusters)),
		0,
	)
	if r1 == 0 {
		if e1 != 0 {
			return -1, fmt.Errorf("UsedDiskSpace (%s): %s", name, error(e1))
		}
		return -1, fmt.Errorf("UsedDiskSpace (%s): %s", name, syscall.EINVAL)
	}
	used := 1 - float64(numberOfFreeClusters)/float64(totalNumberOfClusters)
	return used, nil
}
