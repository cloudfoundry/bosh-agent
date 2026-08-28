package action

import (
	"errors"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type RemovePersistentDiskAction struct {
	settingsService boshsettings.Service
	platform        boshplatform.Platform
}

func NewRemovePersistentDiskAction(settingsService boshsettings.Service, platform boshplatform.Platform) RemovePersistentDiskAction {
	return RemovePersistentDiskAction{
		settingsService: settingsService,
		platform:        platform,
	}
}

func (a RemovePersistentDiskAction) Run(diskCID string) (interface{}, error) {
	err := a.settingsService.LoadSettings()
	if err != nil {
		return nil, bosherr.WrapError(err, "Refreshing the settings")
	}

	// The director sends this immediately before asking the CPI to detach the
	// disk, so release the kernel's device while the disk is still attached.
	// Leaving it behind strands a dead device on the disk's controller slot,
	// which stops a disk later attached to that same slot from being enumerated.
	//
	// A disk with no recorded settings has no device to release, which is normal
	// when a detach is retried or rolled back, so that is not an error.
	allDiskSettings, err := a.settingsService.GetAllPersistentDiskSettings()
	if err != nil {
		return "", bosherr.WrapError(err, "Getting all persistent disk settings")
	}

	if diskSettings, found := allDiskSettings[diskCID]; found {
		if err := a.platform.RemovePersistentDiskDevice(diskSettings); err != nil {
			return "", bosherr.WrapError(err, "Removing persistent disk device")
		}
	}

	if err := a.settingsService.RemovePersistentDiskSettings(diskCID); err != nil {
		return "", bosherr.WrapError(err, "Removing persistent disk hints")
	}

	return map[string]string{}, nil
}

func (a RemovePersistentDiskAction) IsAsynchronous(_ ProtocolVersion) bool {
	return true
}

func (a RemovePersistentDiskAction) IsPersistent() bool {
	return false
}

func (a RemovePersistentDiskAction) IsLoggable() bool {
	return true
}

func (a RemovePersistentDiskAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a RemovePersistentDiskAction) Cancel() error {
	return errors.New("not supported")
}
