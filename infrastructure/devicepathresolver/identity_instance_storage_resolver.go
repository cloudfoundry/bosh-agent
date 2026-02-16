package devicepathresolver

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

// identityInstanceStorageResolver returns device paths as-is from the CPI
type identityInstanceStorageResolver struct {
	devicePathResolver DevicePathResolver
}

// NewIdentityInstanceStorageResolver creates a resolver that uses CPI-provided paths directly
func NewIdentityInstanceStorageResolver(devicePathResolver DevicePathResolver) InstanceStorageResolver {
	return &identityInstanceStorageResolver{
		devicePathResolver: devicePathResolver,
	}
}

func (r *identityInstanceStorageResolver) DiscoverInstanceStorage(devices []boshsettings.DiskSettings) ([]string, error) {
	paths := make([]string, len(devices))
	for i, device := range devices {
		realPath, _, err := r.devicePathResolver.GetRealDevicePath(device)
		if err != nil {
			return nil, bosherr.WrapErrorf(err, "Getting device %s path", device)
		}
		paths[i] = realPath
	}
	return paths, nil
}
