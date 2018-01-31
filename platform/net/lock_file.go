package net

import (
	"os"
	"path/filepath"

	"github.com/charlievieth/fs"
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

func getLockFilePathForDNS(dirProvider boshdir.Provider) string {
	return filepath.Join(dirProvider.BoshDir(), "dns")
}

func LockFileExistsForConfiguredInterfaces(dirProvider boshdirs.Provider) bool {
	lockFile := getLockFilePathForConfiguredInterfaces(dirProvider)

	_, err := os.Stat(lockFile)
	if err == nil || os.IsExist(err) {
		return true
	}

	return false
}

func LockFileExistsForDNS(fs boshsys.FileSystem, dirProvider boshdir.Provider) bool {
	return fs.FileExists(getLockFilePathForDNS(dirProvider))
}

func LockFileExistsForRandomizedPasswords(fs boshsys.FileSystem, dirProvider boshdir.Provider) bool {
	return fs.FileExists(getLockFilePathForRandomizedPasswords(dirProvider))
}

func writeLockFileForConfiguredInterfaces(logger boshlog.Logger, logTag string, dirProvider boshdirs.Provider, fs boshsys.FileSystem) error {
	logger.Info(logTag, "Creating Configured Network Interfaces file...")

	path := getLockFilePathForConfiguredInterfaces(dirProvider)
	if _, err := fs.Stat(path); os.IsNotExist(err) {
		err := writeLockFileHelper(path)
		if err != nil {
			return bosherr.WrapErrorf(err, "Creating configured interfaces file: %s", err)
		}
	}
	return nil
}

func writeLockFileHelper(path string) error {
	f, err := fs.OpenFile(path, os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

func WriteLockFileForDNS(fs boshsys.FileSystem, dirProvider boshdir.Provider) error {
	if err := fs.WriteFileString(getLockFilePathForDNS(dirProvider), ""); err != nil {
		return bosherr.WrapError(err, "Writing DNS password file")
	}
	return nil
}

func WriteLockFileForRandomizedPasswords(fs boshsys.FileSystem, dirProvider boshdir.Provider) error {
	if err := fs.WriteFileString(getLockFilePathForRandomizedPasswords(dirProvider), ""); err != nil {
		return bosherr.WrapError(err, "Writing randomized password file")
	}
	return nil
}
