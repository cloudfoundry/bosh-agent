package disk

import (
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type procMountsSearcher struct {
	fs boshsys.FileSystem
}

func NewProcMountsSearcher(fs boshsys.FileSystem) MountsSearcher {
	return procMountsSearcher{fs}
}

func (s procMountsSearcher) SearchMounts() ([]Mount, error) {
	mountInfo, err := s.fs.ReadFileString("/proc/mounts")
	if err != nil {
		return []Mount{}, bosherr.WrapError(err, "Reading /proc/mounts")
	}

	mountEntries := strings.Split(mountInfo, "\n")
	mounts := make([]Mount, 0, len(mountEntries))
	for _, mountEntry := range mountEntries {
		if mountEntry == "" {
			continue
		}

		mountFields := strings.Fields(mountEntry)

		mounts = append(mounts, Mount{
			PartitionPath: mountFields[0],
			MountPoint:    mountFields[1],
		})
	}

	return mounts, nil
}
