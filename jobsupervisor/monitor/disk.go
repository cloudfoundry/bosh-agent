// +build windows

package monitor

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa364935(v=vs.85).aspx
	procGetDiskFreeSpace = kernel32DLL.MustFindProc("GetDiskFreeSpaceW")
)

// UsedDiskSpace returns the percentage of used disk space for drive name.
func UsedDiskSpace(name string) (float64, error) {
	var (
		sectorsPerCluster     uint32
		bytesPerSector        uint32
		numberOfFreeClusters  uint32
		totalNumberOfClusters uint32
		used                  float64 = -1
	)

	root, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return used, fmt.Errorf("UsedDiskSpace (%s): %s", name, err)
	}
	r1, _, e1 := syscall.Syscall6(procGetDiskFreeSpace.Addr(), 5,
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&sectorsPerCluster)),
		uintptr(unsafe.Pointer(&bytesPerSector)),
		uintptr(unsafe.Pointer(&numberOfFreeClusters)),
		uintptr(unsafe.Pointer(&totalNumberOfClusters)),
		0,
	)
	if err := checkErrno(r1, e1); err != nil {
		return used, fmt.Errorf("UsedDiskSpace (%s): %s", name, err)
	}
	if totalNumberOfClusters > 0 {
		used = 1 - float64(numberOfFreeClusters)/float64(totalNumberOfClusters)
	}
	return used, nil
}
