package action

import (
	"errors"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type RemovePersistentDiskAction struct {
	settingsService boshsettings.Service
}

func NewRemovePersistentDiskAction(settingsService boshsettings.Service) RemovePersistentDiskAction {
	return RemovePersistentDiskAction{
		settingsService: settingsService,
	}
}

func (a RemovePersistentDiskAction) Run(diskCID string) (interface{}, error) {
	err := a.settingsService.LoadSettings()
	if err != nil {
		return nil, bosherr.WrapError(err, "Refreshing the settings")
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
