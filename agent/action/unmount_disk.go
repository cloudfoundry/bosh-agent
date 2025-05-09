package action

import (
	"errors"
	"fmt"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type UnmountDiskAction struct {
	settingsService boshsettings.Service
	platform        boshplatform.Platform
}

func NewUnmountDisk(
	settingsService boshsettings.Service,
	platform boshplatform.Platform,
) (unmountDisk UnmountDiskAction) {
	unmountDisk.settingsService = settingsService
	unmountDisk.platform = platform
	return
}

func (a UnmountDiskAction) IsAsynchronous(_ ProtocolVersion) bool {
	return true
}

func (a UnmountDiskAction) IsPersistent() bool {
	return false
}

func (a UnmountDiskAction) IsLoggable() bool {
	return true
}

func (a UnmountDiskAction) Run(diskID string) (value interface{}, err error) {
	diskSettings, err := a.settingsService.GetPersistentDiskSettings(diskID)
	if err != nil {
		err = bosherr.WrapError(err, "Getting persistent disk settings")
		return
	}

	didUnmount, err := a.platform.UnmountPersistentDisk(diskSettings)
	if err != nil {
		err = bosherr.WrapError(err, "Unmounting persistent disk")
		return
	}

	msg := fmt.Sprintf("Partition of %+v is not mounted", diskSettings)

	if didUnmount {
		msg = fmt.Sprintf("Unmounted partition of %+v", diskSettings)
	}

	type valueType struct {
		Message string `json:"message"`
	}

	value = valueType{Message: msg}
	return
}

func (a UnmountDiskAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a UnmountDiskAction) Cancel() error {
	return errors.New("not supported")
}
