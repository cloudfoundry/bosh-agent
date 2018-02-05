package action

import (
	"errors"
	"fmt"

	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type UnmountDiskAction struct {
	settingsService boshsettings.Service
	platform        boshplatform.Platform
}

type valueType struct {
	Message string `json:"message"`
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
	usedDiskHint := false
	settings := a.settingsService.GetSettings()

	diskSettings, found := settings.PersistentDiskSettings(diskID)

	if !found {
		diskPersistentSettings, found, errorDiskHints := a.settingsService.GetPersistentDiskHint(diskID)

		if errorDiskHints == nil {
			if found {
				diskSettings = diskPersistentSettings
				usedDiskHint = true
			} else {
				err = bosherr.Errorf("Persistent disk with volume id '%s' could not be found", diskID)
				return
			}
		} else {
			err = bosherr.Errorf("Persistent disk with volume id '%s' could not be found.  Lookup error: %v", diskID, errorDiskHints)
			return
		}
	}

	didUnmount, err := a.platform.UnmountPersistentDisk(diskSettings)
	if err != nil {
		err = bosherr.WrapError(err, "Unmounting persistent disk")
		return
	}

	msg := fmt.Sprintf("Partition of %+v is not mounted", diskSettings)

	if didUnmount {
		msg = fmt.Sprintf("Unmounted partition of %+v", diskSettings)
		if usedDiskHint {
			err = a.settingsService.RemovePersistentDiskHint(diskID)
			if err != nil {
				err = bosherr.Errorf("Could not delete disk hint for disk ID %s. Error: %v", diskID, err)
			}
		}
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
