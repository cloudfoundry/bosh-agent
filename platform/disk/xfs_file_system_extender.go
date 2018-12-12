package disk

import (
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type xfsFileSystemExtender struct {
	runner            boshsys.CmdRunner
}

func NewXfsFileSystemExtender(
	runner boshsys.CmdRunner,
) FileSystemExtender {
	return xfsFileSystemExtender{
		runner:            runner,
	}
}

func (e xfsFileSystemExtender) Extend(partitionPath string, size uint64) error {
	_, _, _, err = e.runner.RunCommand(
		"xfs_growfs",
		partitionPath,
		"-D",
		size,
	)

	return err
}
