package bundlecollection

import (
	"os"
	"path"
	"path/filepath"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const (
	fileBundleLogTag = "FileBundle"
)

type FileBundle struct {
	installPath string
	enablePath  string
	fileMode    os.FileMode
	fs          boshsys.FileSystem
	logger      boshlog.Logger
}

func NewFileBundle(
	installPath, enablePath string,
	fileMode os.FileMode,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) FileBundle {
	return FileBundle{
		installPath: installPath,
		enablePath:  enablePath,
		fileMode:    fileMode,
		fs:          fs,
		logger:      logger,
	}
}

func (b FileBundle) Install(sourcePath string) (boshsys.FileSystem, string, error) {
	b.logger.Debug(fileBundleLogTag, "Installing %v", b)

	err := b.fs.Chmod(sourcePath, b.fileMode)
	if err != nil {
		return nil, "", bosherr.WrapError(err, "Setting permissions on source directory")
	}

	err = b.fs.Chown(sourcePath, "root:vcap")
	if err != nil {
		return nil, "", bosherr.WrapError(err, "Setting ownership on source directory")
	}

	err = b.fs.MkdirAll(path.Dir(b.installPath), b.fileMode)
	if err != nil {
		return nil, "", bosherr.WrapError(err, "Creating parent installation directory")
	}

	err = b.fs.Chown(path.Dir(b.installPath), "root:vcap")
	if err != nil {
		return nil, "", bosherr.WrapError(err, "Setting ownership on parent installation directory")
	}

	// Rename MUST be the last possibly-failing operation
	// because IsInstalled() relies on installPath presence.
	err = b.fs.Rename(sourcePath, b.installPath)
	if err != nil {
		return nil, "", bosherr.WrapError(err, "Moving to installation directory")
	}

	return b.fs, b.installPath, nil
}

func (b FileBundle) InstallWithoutContents() (boshsys.FileSystem, string, error) {
	b.logger.Debug(fileBundleLogTag, "Installing without contents %v", b)

	// MkdirAll MUST be the last possibly-failing operation
	// because IsInstalled() relies on installPath presence.
	err := b.fs.MkdirAll(b.installPath, b.fileMode)
	if err != nil {
		return nil, "", bosherr.WrapError(err, "Creating installation directory")
	}
	err = b.fs.Chown(b.installPath, "root:vcap")
	if err != nil {
		return nil, "", bosherr.WrapError(err, "Setting ownership on installation directory")
	}

	return b.fs, b.installPath, nil
}

func (b FileBundle) GetInstallPath() (boshsys.FileSystem, string, error) {
	path := b.installPath
	if !b.fs.FileExists(path) {
		return nil, "", bosherr.Error("install dir does not exist")
	}

	return b.fs, path, nil
}

func (b FileBundle) IsInstalled() (bool, error) {
	return b.fs.FileExists(b.installPath), nil
}

func (b FileBundle) Enable() (boshsys.FileSystem, string, error) {
	b.logger.Debug(fileBundleLogTag, "Enabling %v", b)

	if !b.fs.FileExists(b.installPath) {
		return nil, "", bosherr.Error("bundle must be installed")
	}

	err := b.fs.MkdirAll(filepath.Dir(b.enablePath), b.fileMode)
	if err != nil {
		return nil, "", bosherr.WrapError(err, "failed to create enable dir")
	}

	err = b.fs.Chown(filepath.Dir(b.enablePath), "root:vcap")
	if err != nil {
		return nil, "", bosherr.WrapError(err, "Setting ownership on source directory")
	}

	err = b.fs.Symlink(b.installPath, b.enablePath)
	if err != nil {
		return nil, "", bosherr.WrapError(err, "failed to enable")
	}

	return b.fs, b.enablePath, nil
}

func (b FileBundle) Disable() error {
	b.logger.Debug(fileBundleLogTag, "Disabling %v", b)

	target, err := b.fs.ReadAndFollowLink(b.enablePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return bosherr.WrapError(err, "Reading symlink")
	}

	if target == b.installPath {
		return b.fs.RemoveAll(b.enablePath)
	}

	return nil
}

func (b FileBundle) Uninstall() error {
	b.logger.Debug(fileBundleLogTag, "Uninstalling %v", b)

	// RemoveAll MUST be the last possibly-failing operation
	// because IsInstalled() relies on installPath presence.
	return b.fs.RemoveAll(b.installPath)
}
