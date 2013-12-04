package disk

import (
	bosherr "bosh/errors"
	boshsys "bosh/system"
	"errors"
	"fmt"
	"strings"
	"time"
)

type linuxMounter struct {
	runner            boshsys.CmdRunner
	fs                boshsys.FileSystem
	maxUnmountRetries int
	unmountRetrySleep time.Duration
}

func newLinuxMounter(runner boshsys.CmdRunner, fs boshsys.FileSystem) (mounter linuxMounter) {
	mounter.runner = runner
	mounter.fs = fs
	mounter.maxUnmountRetries = 600
	mounter.unmountRetrySleep = 1 * time.Second
	return
}

func (m linuxMounter) Mount(partitionPath, mountPoint string) (err error) {
	shouldMount, err := m.shouldMount(partitionPath, mountPoint)
	if !shouldMount {
		return
	}

	if err != nil {
		err = bosherr.WrapError(err, "Checking whether partition should be mounted")
		return
	}

	_, _, err = m.runner.RunCommand("mount", partitionPath, mountPoint)
	if err != nil {
		err = bosherr.WrapError(err, "Shelling out to mount")
	}
	return
}

func (m linuxMounter) SwapOn(partitionPath string) (err error) {
	out, _, _ := m.runner.RunCommand("swapon", "-s")

	for i, swapOnLines := range strings.Split(out, "\n") {
		swapOnFields := strings.Fields(swapOnLines)

		switch {
		case i == 0:
			continue
		case len(swapOnFields) == 0:
			continue
		case swapOnFields[0] == partitionPath:
			return
		}
	}

	_, _, err = m.runner.RunCommand("swapon", partitionPath)
	if err != nil {
		err = bosherr.WrapError(err, "Shelling out to swapon")
	}
	return
}

func (m linuxMounter) Unmount(partitionPath string) (didUnmount bool, err error) {
	isMounted, err := m.isMounted(partitionPath)
	if err != nil || !isMounted {
		return
	}

	_, _, err = m.runner.RunCommand("umount", partitionPath)

	for i := 1; i < m.maxUnmountRetries && err != nil; i++ {
		time.Sleep(m.unmountRetrySleep)
		_, _, err = m.runner.RunCommand("umount", partitionPath)
	}

	didUnmount = err == nil
	return
}

func (m linuxMounter) IsMountPoint(path string) (result bool, err error) {
	return m.searchMounts(func(_, mountedMountPoint string) (found bool, err error) {
		if mountedMountPoint == path {
			return true, nil
		}
		return
	})
}

func (m linuxMounter) isMounted(partitionPath string) (isMounted bool, err error) {
	return m.searchMounts(func(mountedPartitionPath, _ string) (found bool, err error) {
		if mountedPartitionPath == partitionPath {
			return true, nil
		}
		return
	})
}

func (m linuxMounter) shouldMount(partitionPath, mountPoint string) (shouldMount bool, err error) {
	isMounted, err := m.searchMounts(func(mountedPartitionPath, mountedMountPoint string) (found bool, err error) {
		switch {
		case mountedPartitionPath == partitionPath && mountedMountPoint == mountPoint:
			found = true
			return
		case mountedPartitionPath == partitionPath && mountedMountPoint != mountPoint:
			err = errors.New(fmt.Sprintf("Device %s is already mounted to %s, can't mount to %s",
				mountedPartitionPath, mountedMountPoint, mountPoint))
			return
		case mountedMountPoint == mountPoint:
			err = errors.New(fmt.Sprintf("Device %s is already mounted to %s, can't mount %s",
				mountedPartitionPath, mountedMountPoint, partitionPath))
			return
		}

		return
	})
	if err != nil {
		return
	}

	shouldMount = !isMounted
	return
}

func (m linuxMounter) searchMounts(mountFieldsFunc func(string, string) (bool, error)) (found bool, err error) {
	mountInfo, err := m.fs.ReadFile("/proc/mounts")
	if err != nil {
		err = bosherr.WrapError(err, "Reading /proc/mounts")
		return
	}

	for _, mountEntry := range strings.Split(mountInfo, "\n") {
		if mountEntry == "" {
			continue
		}

		mountFields := strings.Fields(mountEntry)
		mountedPartitionPath := mountFields[0]
		mountedMountPoint := mountFields[1]

		found, err = mountFieldsFunc(mountedPartitionPath, mountedMountPoint)
		if found || err != nil {
			return
		}
	}
	return
}
