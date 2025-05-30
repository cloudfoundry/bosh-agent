package disk

import (
	"path"
	"strconv"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type Util interface {
	GetFilesContents(diskPath string, fileNames []string) (contents [][]byte, err error)
	GetBlockDeviceSize(diskPath string) (size uint64, err error)
}

type diskUtil struct {
	mounter Mounter
	runner  boshsys.CmdRunner
	fs      boshsys.FileSystem

	logTag string
	logger boshlog.Logger
}

func NewUtil(runner boshsys.CmdRunner, mounter Mounter, fs boshsys.FileSystem, logger boshlog.Logger) Util {
	return diskUtil{
		mounter: mounter,
		runner:  runner,
		fs:      fs,

		logTag: "diskUtil",
		logger: logger,
	}
}

func (util diskUtil) GetFilesContents(diskPath string, fileNames []string) ([][]byte, error) {
	if !util.fs.FileExists(diskPath) {
		return [][]byte{}, bosherr.Errorf("Failed to get file contents, disk path '%s' does not exist", diskPath)
	}

	tempDir, err := util.fs.TempDir("diskutil")
	if err != nil {
		return [][]byte{}, bosherr.WrapError(err, "Creating temporary disk mount point")
	}

	defer func() {
		_ = util.fs.RemoveAll(tempDir) //nolint:errcheck
	}()

	err = util.mounter.Mount(diskPath, tempDir)
	if err != nil {
		return [][]byte{}, bosherr.WrapErrorf(err, "Mounting disk path '%s' to '%s'", diskPath, tempDir)
	}

	util.logger.Debug(util.logTag, "Mounted disk path '%s' to '%s'", diskPath, tempDir)

	contents := [][]byte{}

	for _, fileName := range fileNames {
		diskFilePath := path.Join(tempDir, fileName)

		util.logger.Debug(util.logTag, "Reading contents of '%s'", diskFilePath)

		content, err := util.fs.ReadFile(diskFilePath)
		if err != nil {
			// todo unmount before removing
			if uErr := util.unmount(tempDir); uErr != nil {
				util.logger.Warn(util.logTag, "Failed to unmount temp dir: %s", uErr.Error())
			}
			return [][]byte{}, bosherr.WrapErrorf(err, "Reading from disk file '%s'", diskFilePath)
		}

		util.logger.Debug(util.logTag, "Got contents of %s: %s", diskFilePath, string(content))

		contents = append(contents, content)
	}

	err = util.unmount(tempDir)
	if err != nil {
		return [][]byte{}, err
	}

	return contents, nil
}

func (util diskUtil) GetBlockDeviceSize(diskPath string) (size uint64, err error) {
	stdout, _, _, err := util.runner.RunCommand("lsblk", "--nodeps", "-nb", "-o", "SIZE", diskPath)
	if err != nil {
		util.logger.Error(util.logTag, "Getting the Block Device size of '%s': %s", diskPath, err.Error())
		return 0, err
	}
	deviceSize, err := strconv.Atoi(strings.Trim(stdout, "\n"))
	if err != nil {
		util.logger.Error(util.logTag, "Converting the Block Device size of '%s': %s", diskPath, err.Error())
		return 0, err
	}
	return uint64(deviceSize), nil
}

func (util diskUtil) unmount(tempDir string) error {
	util.logger.Debug(util.logTag, "Unmounting disk path '%s'", tempDir)

	_, err := util.mounter.Unmount(tempDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "Unmounting '%s'", tempDir)
	}

	return nil
}
