package bootonce

import (
	"path/filepath"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const BootonceFileName = "bootonce"

type StartManager struct {
	settings    boshsettings.Service
	fs          boshsys.FileSystem
	dirProvider boshdir.Provider
}

func NewStartManager(
	settings boshsettings.Service,
	fs boshsys.FileSystem,
	dirProvider boshdir.Provider,
) *StartManager {
	return &StartManager{
		settings:    settings,
		fs:          fs,
		dirProvider: dirProvider,
	}
}

func (r *StartManager) CanStart() bool {
	if !r.tmpFsFeatureEnabled() {
		return true
	}

	return !r.fs.FileExists(r.persistentBootoncePath()) || r.fs.FileExists(r.tmpfsBootoncePath())
}

func (r *StartManager) RegisterStart() error {
	if !r.tmpFsFeatureEnabled() {
		return nil
	}

	err := r.fs.WriteFile(r.persistentBootoncePath(), nil)
	if err != nil {
		return err
	}
	return r.fs.WriteFile(r.tmpfsBootoncePath(), nil)
}

func (r *StartManager) tmpFsFeatureEnabled() bool {
	settings := r.settings.GetSettings()
	return settings.TmpFSEnabled()
}

func (r *StartManager) persistentBootoncePath() string {
	return filepath.Join(r.dirProvider.BoshDir(), BootonceFileName)
}

func (r *StartManager) tmpfsBootoncePath() string {
	return filepath.Join(r.dirProvider.CanRestartDir(), BootonceFileName)
}

func touch(fs boshsys.FileSystem, path string) error {
	return fs.WriteFile(path, nil)
}
