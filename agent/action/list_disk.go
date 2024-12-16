package action

import (
	"errors"

	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type ListDiskAction struct {
	settingsService boshsettings.Service
	platform        boshplatform.Platform
	logger          boshlog.Logger
}

func NewListDisk(
	settingsService boshsettings.Service,
	platform boshplatform.Platform,
	logger boshlog.Logger,
) (action ListDiskAction) {
	action.settingsService = settingsService
	action.platform = platform
	action.logger = logger
	return
}

func (a ListDiskAction) IsAsynchronous(version ProtocolVersion) bool {
	return version >= 3
}

func (a ListDiskAction) IsPersistent() bool {
	return false
}

func (a ListDiskAction) IsLoggable() bool {
	return true
}

func (a ListDiskAction) Run() (interface{}, error) {
	err := a.settingsService.LoadSettings()
	if err != nil {
		return nil, bosherr.WrapError(err, "Refreshing the settings")
	}

	diskIDs := make([]string, 0)
	usedIDs := map[string]bool{}

	allPersistentDisks, err := a.settingsService.GetAllPersistentDiskSettings()
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting persistent disk settings")
	}

	for diskID, diskSettings := range allPersistentDisks {
		isMounted, err := a.platform.IsPersistentDiskMounted(diskSettings)
		if err != nil {
			return nil, bosherr.WrapErrorf(err, "Checking whether device %+v is mounted", diskSettings)
		}

		if isMounted {
			if _, present := usedIDs[diskID]; !present {
				diskIDs = append(diskIDs, diskID)
				usedIDs[diskID] = true
			}
		}
	}

	return diskIDs, nil
}

func (a ListDiskAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a ListDiskAction) Cancel() error {
	return errors.New("not supported")
}
