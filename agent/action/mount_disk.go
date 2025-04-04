package action

import (
	"errors"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

type diskMounter interface {
	AdjustPersistentDiskPartitioning(diskSettings boshsettings.DiskSettings, mountPoint string) error
	MountPersistentDisk(diskSettings boshsettings.DiskSettings, mountPoint string) error
}

type MountDiskAction struct {
	settingsService boshsettings.Service
	diskMounter     diskMounter
	dirProvider     boshdirs.Provider
	logger          boshlog.Logger
}

func NewMountDisk(
	settingsService boshsettings.Service,
	diskMounter diskMounter,
	dirProvider boshdirs.Provider,
	logger boshlog.Logger,
) (mountDisk MountDiskAction) {
	mountDisk.settingsService = settingsService
	mountDisk.diskMounter = diskMounter
	mountDisk.dirProvider = dirProvider
	mountDisk.logger = logger
	return
}

func (a MountDiskAction) IsAsynchronous(_ ProtocolVersion) bool {
	return true
}

func (a MountDiskAction) IsPersistent() bool {
	return false
}

func (a MountDiskAction) IsLoggable() bool {
	return true
}

func (a MountDiskAction) Run(diskCid string) (interface{}, error) {
	err := a.settingsService.LoadSettings()
	if err != nil {
		return nil, bosherr.WrapError(err, "Refreshing the settings")
	}

	diskSettings, err := a.settingsService.GetPersistentDiskSettings(diskCid)
	if err != nil {
		return nil, bosherr.WrapError(err, "Reading persistent disk settings")
	}

	mountPoint := a.dirProvider.StoreDir()

	err = a.diskMounter.AdjustPersistentDiskPartitioning(diskSettings, mountPoint)
	if err != nil {
		return nil, bosherr.WrapError(err, "Adjusting persistent disk partitioning")
	}

	err = a.diskMounter.MountPersistentDisk(diskSettings, mountPoint)
	if err != nil {
		return nil, bosherr.WrapError(err, "Mounting persistent disk")
	}

	return map[string]string{}, nil
}

func (a MountDiskAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a MountDiskAction) Cancel() error {
	return errors.New("not supported")
}
