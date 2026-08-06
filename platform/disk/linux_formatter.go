package disk

import (
	"regexp"
	"strconv"
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

func (f linuxFormatter) Format(partitionPath string, fsType FileSystemType) error {
	existingFsType, err := f.GetPartitionFormatType(partitionPath)
	if err != nil {
		return bosherr.WrapError(err, "Checking filesystem format of partition")
	}

	if fsType == FileSystemSwap {
		if existingFsType == FileSystemSwap {
			return err
		}
		// swap is not user-configured, so we're not concerned about reformatting
	} else if existingFsType == FileSystemExt4 || existingFsType == FileSystemXFS {
		// never reformat if it is already formatted in a supported format
		return err
	}

	switch fsType {
	case FileSystemSwap:
		_, _, _, err = f.runner.RunCommand("mkswap", partitionPath)
		if err != nil {
			return bosherr.WrapError(err, "Shelling out to mkswap")
		}

	case FileSystemExt4:
		err = f.makeFileSystemExt4(partitionPath)
		if err != nil {
			if strings.Contains(err.Error(), "apparently in use by the system") {
				err = f.makeFileSystemExt4(partitionPath)
			}
		}
		if err != nil {
			return bosherr.WrapError(err, "Shelling out to mke2fs")
		}

	case FileSystemXFS:
		_, _, _, err = f.runner.RunCommand("mkfs.xfs", partitionPath)
		if err != nil {
			return bosherr.WrapError(err, "Shelling out to mkfs.xfs")
		}
	case FileSystemDefault:
		return nil
	}

	return nil
}

func (f linuxFormatter) GrowFilesystem(partitionPath string) error {
	existingFsType, err := f.GetPartitionFormatType(partitionPath)
	if err != nil {
		return bosherr.WrapError(err, "Checking filesystem format of partition")
	}

	switch existingFsType {
	case FileSystemExt4:
		_, _, _, err := f.runner.RunCommand(
			"resize2fs",
			"-f",
			partitionPath,
		)
		if err != nil {
			return bosherr.WrapError(err, "Failed to grow Ext4 filesystem")
		}

	case FileSystemXFS:
		_, _, _, err = f.runner.RunCommand(
			"xfs_growfs",
			partitionPath,
		)
		if err != nil {
			return bosherr.WrapError(err, "Failed to grow XFS filesystem")
		}
	case FileSystemDefault, FileSystemSwap:
		return nil
	}
	return nil
}

// FilesystemNeedsGrow reports whether the ext4 filesystem on partitionPath is
// significantly smaller than the partition that contains it, indicating a
// previous grow did not complete. "Significantly" uses the same 100MB delta as
// SinglePartitionNeedsResize to avoid false positives from alignment rounding
// between filesystem block groups and partition sector boundaries.
//
// Only ext4 is supported: querying the size of an XFS filesystem requires
// xfs_info, which only works on a mounted filesystem. dumpe2fs reads directly
// from the block device and works regardless of mount state. Unknown or
// unsupported filesystem types return false.
func (f linuxFormatter) FilesystemNeedsGrow(partitionPath string) (bool, error) {
	fsType, err := f.GetPartitionFormatType(partitionPath)
	if err != nil {
		return false, bosherr.WrapError(err, "Checking filesystem format of partition")
	}

	if fsType != FileSystemExt4 {
		return false, nil
	}

	partitionSize, err := f.blockDeviceSize(partitionPath)
	if err != nil {
		return false, err
	}

	fsSize, err := f.ext4FilesystemSize(partitionPath)
	if err != nil {
		return false, err
	}

	return significantlySmallerThan(fsSize, partitionSize, ConvertFromMbToBytes(deltaSize)), nil
}

func (f linuxFormatter) blockDeviceSize(partitionPath string) (uint64, error) {
	stdout, _, _, err := f.runner.RunCommand("blockdev", "--getsize64", partitionPath)
	if err != nil {
		return 0, bosherr.WrapError(err, "Getting block device size")
	}

	size, err := strconv.ParseUint(strings.TrimSpace(stdout), 10, 64)
	if err != nil {
		return 0, bosherr.WrapError(err, "Parsing block device size")
	}

	return size, nil
}

func (f linuxFormatter) ext4FilesystemSize(partitionPath string) (uint64, error) {
	stdout, _, _, err := f.runner.RunCommand("dumpe2fs", "-h", partitionPath)
	if err != nil {
		return 0, bosherr.WrapError(err, "Reading ext4 superblock")
	}

	blockCount, err := parseDumpe2fsField(stdout, "Block count:")
	if err != nil {
		return 0, err
	}

	blockSize, err := parseDumpe2fsField(stdout, "Block size:")
	if err != nil {
		return 0, err
	}

	return blockCount * blockSize, nil
}

func parseDumpe2fsField(stdout, field string) (uint64, error) {
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, field) {
			value := strings.TrimSpace(strings.TrimPrefix(line, field))
			return strconv.ParseUint(value, 10, 64)
		}
	}

	return 0, bosherr.Errorf("Field %q not found in dumpe2fs output", field)
}

func (f linuxFormatter) makeFileSystemExt4(partitionPath string) error {
	var err error
	if f.fs.FileExists("/sys/fs/ext4/features/lazy_itable_init") {
		_, _, _, err = f.runner.RunCommand("mke2fs", "-t", string(FileSystemExt4), "-j", "-E", "lazy_itable_init=1", partitionPath)
	} else {
		_, _, _, err = f.runner.RunCommand("mke2fs", "-t", string(FileSystemExt4), "-j", partitionPath)
	}
	return err
}

func (f linuxFormatter) GetPartitionFormatType(partitionPath string) (FileSystemType, error) {
	stdout, stderr, exitStatus, err := f.runner.RunCommand("blkid", "-p", partitionPath)

	if err != nil {
		if exitStatus == 2 && stderr == "" {
			// in that case we expect the device not to have any file system
			return "", nil
		}
		return "", err
	}

	re := regexp.MustCompile(" TYPE=\"([^\"]+)\"")
	match := re.FindStringSubmatch(stdout)

	if nil == match {
		return "", nil
	}

	return FileSystemType(match[1]), nil
}
