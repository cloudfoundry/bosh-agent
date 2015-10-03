package action

import (
	"errors"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type diskMounter interface {
	MountPersistentDisk(diskSettings boshsettings.DiskSettings, mountPoint string) error
}

type mountPoints interface {
	IsMountPoint(string) (bool, error)
}

type MountDiskAction struct {
	settingsService boshsettings.Service
	diskMounter     diskMounter
	mountPoints     mountPoints
	dirProvider     boshdirs.Provider
}

func NewMountDisk(
	settingsService boshsettings.Service,
	diskMounter diskMounter,
	mountPoints mountPoints,
	dirProvider boshdirs.Provider,
) (mountDisk MountDiskAction) {
	mountDisk.settingsService = settingsService
	mountDisk.diskMounter = diskMounter
	mountDisk.mountPoints = mountPoints
	mountDisk.dirProvider = dirProvider
	return
}

func (a MountDiskAction) IsAsynchronous() bool {
	return true
}

func (a MountDiskAction) IsPersistent() bool {
	return false
}

func (a MountDiskAction) Run(diskCid string) (interface{}, error) {
	err := a.settingsService.LoadSettings()
	if err != nil {
		return nil, bosherr.WrapError(err, "Refreshing the settings")
	}

	settings := a.settingsService.GetSettings()

	diskSettings, found := settings.PersistentDiskSettings(diskCid)
	if !found {
		return nil, bosherr.Errorf("Persistent disk with volume id '%s' could not be found", diskCid)
	}

	mountPoint := a.dirProvider.StoreDir()

	isMountPoint, err := a.mountPoints.IsMountPoint(mountPoint)
	if err != nil {
		return nil, bosherr.WrapError(err, "Checking mount point")
	}
	if isMountPoint {
		mountPoint = a.dirProvider.StoreMigrationDir()
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
