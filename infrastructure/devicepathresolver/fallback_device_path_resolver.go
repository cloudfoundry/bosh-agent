package devicepathresolver

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type FallbackDevicePathResolver struct {
	primary   DevicePathResolver
	secondary DevicePathResolver
	logger    boshlog.Logger
	logTag    string
}

func NewFallbackDevicePathResolver(
	primary DevicePathResolver,
	secondary DevicePathResolver,
	logger boshlog.Logger,
) DevicePathResolver {
	return FallbackDevicePathResolver{
		primary:   primary,
		secondary: secondary,
		logger:    logger,
		logTag:    "fallbackDevicePathResolver",
	}
}

func (r FallbackDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	realPath, _, err := r.primary.GetRealDevicePath(diskSettings)
	if err == nil {
		r.logger.Debug(r.logTag, "Primary resolver resolved disk %+v as '%s'", diskSettings, realPath)
		return realPath, false, nil
	}

	r.logger.Debug(r.logTag,
		"Primary resolver failed for disk %+v: %s. Trying secondary resolver.",
		diskSettings, err.Error(),
	)

	realPath, timeout, err := r.secondary.GetRealDevicePath(diskSettings)
	if err != nil {
		return "", timeout, bosherr.WrapError(err, "Secondary resolver also failed")
	}

	r.logger.Debug(r.logTag, "Secondary resolver resolved disk %+v as '%s'", diskSettings, realPath)
	return realPath, false, nil
}
