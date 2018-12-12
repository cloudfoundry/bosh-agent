package disk

import (
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type ext4FileSystemExtender struct {
	runner            boshsys.CmdRunner
}

func NewExt4FileSystemExtender(
	runner boshsys.CmdRunner,
) FileSystemExtender {
	return ext4FileSystemExtender{
		runner:            runner,
	}
}

func (e ext4FileSystemExtender) Extend(partitionPath string, size uint64) error{
	_, _, _, err = e.runner.RunCommand(
		"resize2fs",
		"-f",
		partitionPath,
	)

	return err
}
