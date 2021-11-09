package disk

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Mounter

type Mounter interface {
	Mount(partitionPath, mountPoint string, mountOptions ...string) (err error)
	MountTmpfs(mountPoint string, size string) (err error)
	MountFilesystem(partitionPath, mountPoint, fstype string, mountOptions ...string) (err error)
	Unmount(partitionOrMountPoint string) (didUnmount bool, err error)

	RemountAsReadonly(mountPoint string) (err error)
	Remount(fromMountPoint, toMountPoint string, mountOptions ...string) (err error)
	RemountInPlace(mountPoint string, mountOptions ...string) (err error)

	SwapOn(partitionPath string) (err error)

	IsMountPoint(path string) (parititionPath string, result bool, err error)
	IsMounted(devicePathOrMountPoint string) (result bool, err error)
}
