package devicepathresolver

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type virtioDevicePathResolver struct {
	volumeIDDevicePathResolver DevicePathResolver
	idDevicePathResolver       DevicePathResolver
	mappedDevicePathResolver   DevicePathResolver
	logger                     boshlog.Logger
	logTag                     string
}

func NewVirtioDevicePathResolver(
	volumeIDDevicePathResolver DevicePathResolver,
	idDevicePathResolver DevicePathResolver,
	mappedDevicePathResolver DevicePathResolver,
	logger boshlog.Logger,
) DevicePathResolver {
	return virtioDevicePathResolver{
		volumeIDDevicePathResolver: volumeIDDevicePathResolver,
		idDevicePathResolver:       idDevicePathResolver,
		mappedDevicePathResolver:   mappedDevicePathResolver,
		logger:                     logger,
		logTag:                     "virtioDevicePathResolver",
	}
}

func (vpr virtioDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	realPath, timeout, err := vpr.volumeIDDevicePathResolver.GetRealDevicePath(diskSettings)
	if err == nil {
		vpr.logger.Debug(vpr.logTag, "Resolved disk %+v by VolumeID as '%s'", diskSettings, realPath)
		return realPath, false, nil
	}

	vpr.logger.Debug(vpr.logTag,
		"Failed to get device real path by VolumeID: '%s'. Error: '%s', timeout: '%t'",
		diskSettings.VolumeID,
		err.Error(),
		timeout,
	)

	realPath, timeout, err = vpr.idDevicePathResolver.GetRealDevicePath(diskSettings)
	if err == nil {
		vpr.logger.Debug(vpr.logTag, "Resolved disk %+v by ID as '%s'", diskSettings, realPath)
		return realPath, false, nil
	}

	vpr.logger.Debug(vpr.logTag,
		"Failed to get device real path by disk ID: '%s'. Error: '%s', timeout: '%t'",
		diskSettings.ID,
		err.Error(),
		timeout,
	)

	vpr.logger.Debug(vpr.logTag, "Using mapped resolver to get device real path")

	realPath, timeout, err = vpr.mappedDevicePathResolver.GetRealDevicePath(diskSettings)
	if err != nil {
		return "", timeout, bosherr.WrapError(err, "Resolving mapped device path")
	}

	return realPath, false, nil
}
