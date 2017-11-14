package net

import (
	"os"
	"path/filepath"

	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
)

func getLockFilePathForConfiguredInterfaces(dirProvider boshdirs.Provider) string {
	return filepath.Join(dirProvider.BoshDir(), "configured_interfaces.txt")
}

func getLockFilePathForRandomizedPasswords(dirProvider boshdir.Provider) string {
	return filepath.Join(dirProvider.BoshDir(), "randomized_passwords")
}

func LockFileExistsForConfiguredInterfaces(dirProvider boshdirs.Provider) bool {
	lockFile := getLockFilePathForConfiguredInterfaces(dirProvider)

	_, err := os.Stat(lockFile)
	if err == nil || os.IsExist(err) {
		return true
	}

	return false
}

func LockFileExistsForRandomizedPasswords(fs boshsys.FileSystem, dirProvider boshdir.Provider) bool {
	return fs.FileExists(getLockFilePathForRandomizedPasswords(dirProvider))
}

func writeLockFileForConfiguredInterfaces(logger boshlog.Logger, logTag string, dirProvider boshdirs.Provider, fs boshsys.FileSystem) error {
	logger.Info(logTag, "Creating Configured Network Interfaces file...")

	path := getLockFilePathForConfiguredInterfaces(dirProvider)
	if _, err := fs.Stat(path); os.IsNotExist(err) {
		f, err := fs.OpenFile(path, os.O_CREATE, 0644)
		if err != nil {
			return bosherr.WrapErrorf(err, "Creating configured interfaces file: %s", err)
		}
		f.Close()
	}
	return nil
}

func WriteLockFileForRandomizedPasswords(fs boshsys.FileSystem, dirProvider boshdir.Provider) error {
	if err := fs.WriteFileString(getLockFilePathForRandomizedPasswords(dirProvider), ""); err != nil {
		return bosherr.WrapError(err, "Writing randomized password file")
	}
	return nil
}