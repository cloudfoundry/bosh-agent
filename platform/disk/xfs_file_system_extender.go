package disk

import (
	"strconv"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type xfsFileSystemExtender struct {
	runner boshsys.CmdRunner
}

func NewXfsFileSystemExtender(
	runner boshsys.CmdRunner,
) FileSystemExtender {
	return xfsFileSystemExtender{
		runner: runner,
	}
}

func (e xfsFileSystemExtender) Extend(partitionPath string, size uint64) error {
	s := strconv.FormatUint(size, 10)
	_, _, _, err := e.runner.RunCommand(
		"xfs_growfs",
		partitionPath,
		"-D",
		s,
	)

	return err
}
