package bundlecollection

import (
	"path"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const BundleSetupTimeout = 2 * time.Minute

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

	startTime := b.timeProvider.Now()

	for b.timeProvider.Since(startTime) < BundleSetupTimeout {
		err = b.fs.Rename(sourcePath, b.installPath)
		if err == nil {
			return b.fs, b.installPath, nil
		}
		b.timeProvider.Sleep(time.Second * 5)
	}

	return nil, "", bosherr.WrapError(err, "Moving to installation directory")
}

func (b FileBundle) Uninstall() error {
	b.logger.Debug(fileBundleLogTag, "Uninstalling %v", b)

	// RemoveAll MUST be the last possibly-failing operation
	// because IsInstalled() relies on installPath presence.

	var err error
	startTime := b.timeProvider.Now()

	for b.timeProvider.Since(startTime) < BundleSetupTimeout {
		err = b.fs.RemoveAll(b.installPath)
		if err == nil {
			return nil
		}
		b.timeProvider.Sleep(time.Second * 5)
	}

	return err
}
