package disk

import "strings"

type Mount struct {
	PartitionPath string
	MountPoint    string
}

func (m Mount) IsRoot() bool {
	return m.MountPoint == "/" && strings.HasPrefix(m.PartitionPath, "/dev/")
}

type MountsSearcher interface {
	SearchMounts() ([]Mount, error)
}
