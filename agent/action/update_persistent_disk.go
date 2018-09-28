package action

import (
	"errors"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type UpdatePersistentDiskAction struct {
	settingsService boshsettings.Service
}

func NewUpdatePersistentDiskAction(settingsService boshsettings.Service) UpdatePersistentDiskAction {
	return UpdatePersistentDiskAction{
		settingsService: settingsService,
	}
}

func (a UpdatePersistentDiskAction) Run(diskCID string, diskHint interface{}) (interface{}, error) {
	err := a.settingsService.LoadSettings()
	if err != nil {
		return nil, bosherr.WrapError(err, "Refreshing the settings")
	}

	currentSettings := a.settingsService.GetSettings()

	diskSetting := currentSettings.PersistentDiskSettingsFromHint(diskCID, diskHint)
	if err := a.settingsService.SavePersistentDiskHint(diskSetting); err != nil {
		return "", bosherr.WrapError(err, "Saving persistent disk hints")
	}

	return "updated_persistent_disk", nil
}

func (a UpdatePersistentDiskAction) IsAsynchronous(_ ProtocolVersion) bool {
	return false
}

func (a UpdatePersistentDiskAction) IsPersistent() bool {
	return false
}

func (a UpdatePersistentDiskAction) IsLoggable() bool {
	return true
}

func (a UpdatePersistentDiskAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a UpdatePersistentDiskAction) Cancel() error {
	return errors.New("not supported")
}
