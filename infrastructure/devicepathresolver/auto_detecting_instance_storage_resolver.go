package devicepathresolver

import (
	"strings"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

// autoDetectingInstanceStorageResolver automatically detects whether to use
// NVMe-specific logic or identity resolution based on device paths from the CPI.
// If any device path starts with "/dev/nvme", it uses symlink-based NVMe discovery.
// Otherwise, it uses the CPI-provided paths directly (identity resolution).
type autoDetectingInstanceStorageResolver struct {
	fs                        boshsys.FileSystem
	devicePathResolver        DevicePathResolver
	logger                    boshlog.Logger
	managedDiskSymlinkPattern string
	nvmeDevicePattern         string
	nvmeResolver              InstanceStorageResolver
	identityResolver          InstanceStorageResolver
	resolverInitialized       bool
	useNVMeResolver           bool
}

// NewAutoDetectingInstanceStorageResolver creates a resolver that automatically
// detects NVMe instances based on device paths from the CPI.
func NewAutoDetectingInstanceStorageResolver(
	fs boshsys.FileSystem,
	devicePathResolver DevicePathResolver,
	logger boshlog.Logger,
	managedDiskSymlinkPattern string,
	nvmeDevicePattern string,
) InstanceStorageResolver {
	if managedDiskSymlinkPattern == "" {
		managedDiskSymlinkPattern = AWSEBSSymlinkPattern
	}
	if nvmeDevicePattern == "" {
		nvmeDevicePattern = DefaultNVMeDevicePattern
	}

	return &autoDetectingInstanceStorageResolver{
		fs:                        fs,
		devicePathResolver:        devicePathResolver,
		logger:                    logger,
		managedDiskSymlinkPattern: managedDiskSymlinkPattern,
		nvmeDevicePattern:         nvmeDevicePattern,
		resolverInitialized:       false,
	}
}

func (r *autoDetectingInstanceStorageResolver) DiscoverInstanceStorage(devices []boshsettings.DiskSettings) ([]string, error) {
	if len(devices) == 0 {
		return []string{}, nil
	}

	// Auto-detect on first call by checking if any device path starts with /dev/nvme
	if !r.resolverInitialized {
		r.useNVMeResolver = r.detectNVMeDevices(devices)

		if r.useNVMeResolver {
			r.logger.Info("AutoDetectingInstanceStorageResolver",
				"Detected NVMe device paths from CPI - using symlink-based NVMe instance storage discovery")
			r.nvmeResolver = NewNVMeSymlinkFilteringResolver(
				r.fs,
				r.logger,
				r.managedDiskSymlinkPattern,
				r.nvmeDevicePattern,
			)
		} else {
			r.logger.Info("AutoDetectingInstanceStorageResolver",
				"Detected non-NVMe device paths from CPI - using identity resolution")
			r.identityResolver = NewIdentityInstanceStorageResolver(r.devicePathResolver)
		}

		r.resolverInitialized = true
	}

	if r.useNVMeResolver {
		return r.nvmeResolver.DiscoverInstanceStorage(devices)
	}
	return r.identityResolver.DiscoverInstanceStorage(devices)
}

// detectNVMeDevices checks if any device path from the CPI starts with /dev/nvme
// This matches the CPI's logic: if current_disk =~ /^\/dev\/nvme/
func (r *autoDetectingInstanceStorageResolver) detectNVMeDevices(devices []boshsettings.DiskSettings) bool {
	for _, device := range devices {
		if strings.HasPrefix(device.Path, "/dev/nvme") {
			r.logger.Debug("AutoDetectingInstanceStorageResolver",
				"Detected NVMe from CPI-provided path: %s", device.Path)
			return true
		}
	}
	return false
}
