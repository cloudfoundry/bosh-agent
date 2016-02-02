package action

import (
	"errors"
	"fmt"

	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

const logTag string = "mount_disk_action"

type diskMounter interface {
	MountPersistentDisk(diskSettings boshsettings.DiskSettings, mountPoint string) error
	FindDeviceMatchingMountPoint(mountPoint string) (string, bool, error)
}

type MountDiskAction struct {
	settingsService    boshsettings.Service
	diskMounter        diskMounter
	devicePathResolver boshdpresolv.DevicePathResolver
	dirProvider        boshdirs.Provider
	logger             boshlog.Logger
}

func NewMountDisk(
	settingsService boshsettings.Service,
	diskMounter diskMounter,
	devicePathResolver boshdpresolv.DevicePathResolver,
	dirProvider boshdirs.Provider,
	logger boshlog.Logger,
) (mountDisk MountDiskAction) {
	mountDisk.settingsService = settingsService
	mountDisk.diskMounter = diskMounter
	mountDisk.devicePathResolver = devicePathResolver
	mountDisk.dirProvider = dirProvider
	mountDisk.logger = logger
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

	realPath, _, err := a.devicePathResolver.GetRealDevicePath(diskSettings)
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting real device path")
	}

	devicePath, isMountPoint, err := a.diskMounter.FindDeviceMatchingMountPoint(mountPoint)
	if err != nil {
		return nil, bosherr.WrapError(err, "Checking mount point")
	}
	a.logger.Info(logTag, "realPath = %s, devicePath = %s, isMountPoint = %s", realPath, devicePath, isMountPoint)

	if isMountPoint {
		if realPath == devicePath || fmt.Sprint(realPath+"1") == devicePath {
			return map[string]string{}, nil
		}

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
