package disk

import (
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type cmdMountsSearcher struct {
	runner boshsys.CmdRunner
}

func NewCmdMountsSearcher(runner boshsys.CmdRunner) MountsSearcher {
	return cmdMountsSearcher{runner}
}

func (s cmdMountsSearcher) SearchMounts() ([]Mount, error) {
	stdout, _, _, err := s.runner.RunCommand("mount")
	if err != nil {
		return []Mount{}, bosherr.WrapError(err, "Running mount")
	}

	mountEntries := strings.Split(stdout, "\n")
	mounts := make([]Mount, 0, len(mountEntries))
	for _, mountEntry := range mountEntries {
		if mountEntry == "" {
			continue
		}

		// e.g. '/dev/sda on /boot type ext2 (rw)'
		mountFields := strings.Fields(mountEntry)

		mounts = append(mounts, Mount{
			PartitionPath: mountFields[0],
			MountPoint:    mountFields[2],
		})
	}

	return mounts, nil
}
