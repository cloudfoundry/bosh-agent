package devicepathresolver

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type scsi struct {
	scsiVolumeID 	DevicePathResolver
	scsiID          DevicePathResolver
}

func NewScsi(
	scsiVolumeID DevicePathResolver,
	scsiID DevicePathResolver,
) DevicePathResolver {
	return scsi{
		scsiVolumeID: scsiVolumeID,
		scsiID: scsiID,
	}
}

func (scsiResolver scsi) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	var realPath string
	var timeout bool
	var err error

	if len(diskSettings.ID) > 0 {
		realPath, timeout, err = scsiResolver.scsiID.GetRealDevicePath(diskSettings)
	} else if len(diskSettings.VolumeID) > 0 {
		realPath, timeout, err = scsiResolver.scsiVolumeID.GetRealDevicePath(diskSettings)
	}
	return realPath, timeout, err
}
