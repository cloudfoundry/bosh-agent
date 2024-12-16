package devicepathresolver

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type multipathDevicePathResolver struct {
	idDevicePathResolver    DevicePathResolver
	iscsiDevicePathResolver DevicePathResolver
	logger                  boshlog.Logger
	logTag                  string
}

func NewMultipathDevicePathResolver(
	idDevicePathResolver DevicePathResolver,
	iscsiDevicePathResolver DevicePathResolver,
	logger boshlog.Logger,
) DevicePathResolver {
	return multipathDevicePathResolver{
		idDevicePathResolver:    idDevicePathResolver,
		iscsiDevicePathResolver: iscsiDevicePathResolver,
		logger:                  logger,
		logTag:                  "multipathDevicePathResolver",
	}
}

func (mpr multipathDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	realPath, timeout, err := mpr.idDevicePathResolver.GetRealDevicePath(diskSettings)
	if err == nil {
		mpr.logger.Debug(mpr.logTag, "Resolved disk %+v by ID as '%s'", diskSettings, realPath)
		return realPath, false, nil
	}

	if timeout {
		return "", timeout, bosherr.WrapError(err, "Resolving id device path")
	}

	mpr.logger.Debug(mpr.logTag,
		"Failed to get device real path by disk ID: '%s'. Error: '%s', timeout: '%t'",
		diskSettings.ID,
		err.Error(),
		timeout,
	)

	mpr.logger.Debug(mpr.logTag, "Using iSCSI resolver to get device real path")

	realPath, timeout, err = mpr.iscsiDevicePathResolver.GetRealDevicePath(diskSettings)
	if err != nil {
		return "", timeout, bosherr.WrapError(err, "Resolving mapped device path")
	}

	return realPath, timeout, nil
}
