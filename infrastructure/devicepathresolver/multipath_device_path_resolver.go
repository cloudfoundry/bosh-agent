package devicepathresolver

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type multipathDevicePathResolver struct {
	usePreformattedPersistentDisk bool
}

func NewMultipathDevicePathResolver(usePreformattedPersistentDisk bool) DevicePathResolver {
	return multipathDevicePathResolver{usePreformattedPersistentDisk: usePreformattedPersistentDisk}
}

func (r multipathDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	if r.usePreformattedPersistentDisk {
		return diskSettings.Path, false, nil
	} else {
		return diskSettings.Path + "-part", false, nil
	}
}
