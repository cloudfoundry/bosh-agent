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
	// QuietContent: /proc/mounts is read on every heartbeat; dumping its full
	// content at DEBUG floods logs (hundreds of container overlay mounts per
	// cell). Keep the "Reading file" trace, drop the content dump.
	mountInfo, err := s.fs.ReadFileWithOpts("/proc/mounts", boshsys.ReadOpts{QuietContent: true})
	if err != nil {
		return []Mount{}, bosherr.WrapError(err, "Reading /proc/mounts")
	}

	mountEntries := strings.Split(string(mountInfo), "\n")
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
