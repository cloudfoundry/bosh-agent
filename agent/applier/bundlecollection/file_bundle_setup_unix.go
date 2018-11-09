// +build !windows

package bundlecollection

import (
	"path"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

func (b FileBundle) Install(sourcePath string) (string, error) {
	b.logger.Debug(fileBundleLogTag, "Installing %v", b)

	err := b.fs.Chmod(sourcePath, b.fileMode)
	if err != nil {
		return "", bosherr.WrapError(err, "Setting permissions on source directory")
	}

	err = b.fs.Chown(sourcePath, "root:vcap")
	if err != nil {
		return "", bosherr.WrapError(err, "Setting ownership on source directory")
	}

	err = b.fs.MkdirAll(path.Dir(b.installPath), b.fileMode)
	if err != nil {
		return "", bosherr.WrapError(err, "Creating parent installation directory")
	}

	err = b.fs.Chown(path.Dir(b.installPath), "root:vcap")
	if err != nil {
		return "", bosherr.WrapError(err, "Setting ownership on parent installation directory")
	}

	// Rename/CopyDir MUST be the last possibly-failing operation
	// because IsInstalled() relies on installPath presence.
	if err := b.fs.Rename(sourcePath, b.installPath); err != nil {
		// Rename can fail if the source and install path are on two different
		// mountpoints. We fall back to copying the directory and cleaning up
		// afterwards in this case.
		if err := b.fs.CopyDir(sourcePath, b.installPath); err != nil {
			return "", bosherr.WrapError(err, "Moving to installation directory")
		}
		if err := b.fs.RemoveAll(sourcePath); err != nil {
			return "", bosherr.WrapError(err, "Deleting copied source directory")
		}
	}

	return b.installPath, nil
}

func (b FileBundle) Uninstall() error {
	b.logger.Debug(fileBundleLogTag, "Uninstalling %v", b)

	// RemoveAll MUST be the last possibly-failing operation
	// because IsInstalled() relies on installPath presence.
	return b.fs.RemoveAll(b.installPath)
}
