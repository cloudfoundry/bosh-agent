package action

import (
	"errors"

	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type AddDynamicDiskAction struct {
	settingsService boshsettings.Service
	platform        boshplatform.Platform
}

func NewAddDynamicDiskAction(settingsService boshsettings.Service, platform boshplatform.Platform) AddDynamicDiskAction {
	return AddDynamicDiskAction{
		settingsService: settingsService,
		platform:        platform,
	}
}

func (a AddDynamicDiskAction) Run(diskCID string, diskHint interface{}) (interface{}, error) {
	err := a.settingsService.LoadSettings()
	if err != nil {
		return nil, bosherr.WrapError(err, "Refreshing the settings")
	}

	currentSettings := a.settingsService.GetSettings()

	diskSetting := currentSettings.PersistentDiskSettingsFromHint(diskCID, diskHint)
	if err := a.platform.SetupDynamicDisk(diskSetting); err != nil {
		return "", bosherr.WrapError(err, "Setting up dynamic disk")
	}

	return map[string]string{}, nil
}

func (a AddDynamicDiskAction) IsAsynchronous(_ ProtocolVersion) bool {
	return true
}

func (a AddDynamicDiskAction) IsPersistent() bool {
	return false
}

func (a AddDynamicDiskAction) IsLoggable() bool {
	return true
}

func (a AddDynamicDiskAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a AddDynamicDiskAction) Cancel() error {
	return errors.New("not supported")
}
