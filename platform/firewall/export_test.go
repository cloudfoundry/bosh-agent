//go:build linux

package firewall

func InjectCgroupResolvers(manager Manager, path func() (string, error), inode func(string) (uint64, error)) {
	concrete := manager.(*NftablesFirewall)
	concrete.currentCgroup = path
	concrete.cgroupInode = inode
}
