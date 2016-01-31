package disk

import (
	"fmt"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type linuxFormatter struct {
	runner boshsys.CmdRunner
	fs     boshsys.FileSystem
}

func NewLinuxFormatter(runner boshsys.CmdRunner, fs boshsys.FileSystem) Formatter {
	return linuxFormatter{
		runner: runner,
		fs:     fs,
	}
}

func (f linuxFormatter) Format(partitionPath string, fsType FileSystemType) (err error) {
	hasGivenType, err := f.partitionHasGivenType(partitionPath, fsType)
	if err != nil {
		return bosherr.WrapError(err, "Checking partition type")
	}

	if hasGivenType {
		return
	}

	switch fsType {
	case FileSystemSwap:
		_, _, _, err = f.runner.RunCommand("mkswap", partitionPath)
		if err != nil {
			err = bosherr.WrapError(err, "Shelling out to mkswap")
		}

	case FileSystemExt4:
		if f.fs.FileExists("/sys/fs/ext4/features/lazy_itable_init") {
			_, _, _, err = f.runner.RunCommand("mke2fs", "-t", "ext4", "-j", "-E", "lazy_itable_init=1", partitionPath)
		} else {
			_, _, _, err = f.runner.RunCommand("mke2fs", "-t", "ext4", "-j", partitionPath)
		}
		if err != nil {
			err = bosherr.WrapError(err, "Shelling out to mke2fs")
		}
	}
	return
}

func (f linuxFormatter) partitionHasGivenType(partitionPath string, fsType FileSystemType) (bool, error) {
	stdout, _, _, err := f.runner.RunCommand("blkid", "-p", partitionPath)
	if err != nil {
		return false, bosherr.WrapError(err, "Shelling out to blkid")
	}

	return strings.Contains(stdout, fmt.Sprintf(` TYPE="%s"`, fsType)), nil
}
