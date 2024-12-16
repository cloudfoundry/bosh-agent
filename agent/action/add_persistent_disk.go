package action

import (
	"errors"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type AddPersistentDiskAction struct {
	settingsService boshsettings.Service
}

func NewAddPersistentDiskAction(settingsService boshsettings.Service) AddPersistentDiskAction {
	return AddPersistentDiskAction{
		settingsService: settingsService,
	}
}

func (a AddPersistentDiskAction) Run(diskCID string, diskHint interface{}) (interface{}, error) {
	err := a.settingsService.LoadSettings()
	if err != nil {
		return nil, bosherr.WrapError(err, "Refreshing the settings")
	}

	currentSettings := a.settingsService.GetSettings()

	diskSetting := currentSettings.PersistentDiskSettingsFromHint(diskCID, diskHint)
	if err := a.settingsService.SavePersistentDiskSettings(diskSetting); err != nil {
		return "", bosherr.WrapError(err, "Saving persistent disk hints")
	}

	return map[string]string{}, nil
}

func (a AddPersistentDiskAction) IsAsynchronous(_ ProtocolVersion) bool {
	return true
}

func (a AddPersistentDiskAction) IsPersistent() bool {
	return false
}

func (a AddPersistentDiskAction) IsLoggable() bool {
	return true
}

func (a AddPersistentDiskAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a AddPersistentDiskAction) Cancel() error {
	return errors.New("not supported")
}
