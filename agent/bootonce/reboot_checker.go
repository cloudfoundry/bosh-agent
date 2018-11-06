package bootonce

import (
	"path/filepath"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type RebootChecker struct {
	settings    boshsettings.Service
	fs          boshsys.FileSystem
	dirProvider boshdir.Provider
}

func NewRebootChecker(
	settings boshsettings.Service,
	fs boshsys.FileSystem,
	dirProvider boshdir.Provider,
) *RebootChecker {
	return &RebootChecker{
		settings:    settings,
		fs:          fs,
		dirProvider: dirProvider,
	}
}

func (r *RebootChecker) CanReboot() (bool, error) {
	if !r.tmpFsFeatureEnabled() {
		return true, nil
	}

	path := filepath.Join(r.dirProvider.BoshDir(), "bootonce")
	return checkAndMark(r.fs, path)
}

func (r *RebootChecker) tmpFsFeatureEnabled() bool {
	settings := r.settings.GetSettings()
	return settings.Env.Bosh.JobDir.TmpFs
}

func checkAndMark(fs boshsys.FileSystem, path string) (bool, error) {
	if fs.FileExists(path) {
		return false, nil
	}

	return true, touch(fs, path)
}

func touch(fs boshsys.FileSystem, path string) error {
	return fs.WriteFile(path, nil)
}
