package devicepathresolver

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const (
	NVMeDevicePattern = "/dev/nvme*n1"
)

type SymlinkDeviceResolver struct {
	fs     boshsys.FileSystem
	logger boshlog.Logger
	logTag string
}

// NewSymlinkDeviceResolver creates a new symlink device resolver.
func NewSymlinkDeviceResolver(
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) *SymlinkDeviceResolver {
	return &SymlinkDeviceResolver{
		fs:     fs,
		logger: logger,
		logTag: "SymlinkDeviceResolver",
	}
}

// ResolveSymlinksToDevices resolves all symlinks matching the given pattern
// and returns a map of resolved device paths -> symlink paths.
func (r *SymlinkDeviceResolver) ResolveSymlinksToDevices(symlinkPattern string) (map[string]string, error) {
	symlinks, err := r.fs.Glob(symlinkPattern)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Globbing symlinks with pattern '%s'", symlinkPattern)
	}

	result := make(map[string]string)
	for _, symlink := range symlinks {
		absPath, err := r.fs.ReadAndFollowLink(symlink)
		if err != nil {
			r.logger.Debug(r.logTag, "Could not resolve symlink %s: %s", symlink, err.Error())
			continue
		}

		r.logger.Debug(r.logTag, "Resolved symlink: %s -> %s", symlink, absPath)
		result[absPath] = symlink
	}

	return result, nil
}

// GetDevicesByPattern returns all devices matching the given pattern.
func (r *SymlinkDeviceResolver) GetDevicesByPattern(devicePattern string) ([]string, error) {
	devices, err := r.fs.Glob(devicePattern)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Globbing devices with pattern '%s'", devicePattern)
	}

	r.logger.Debug(r.logTag, "Found devices matching '%s': %v", devicePattern, devices)
	return devices, nil
}

// FilterDevices returns devices that are NOT in the exclusion map.
// This is used to filter out IaaS-managed volumes (EBS, Azure Managed Disks, etc.)
// from the list of all NVMe devices, leaving only instance/ephemeral storage.
func (r *SymlinkDeviceResolver) FilterDevices(allDevices []string, excludeDevices map[string]string) []string {
	var filtered []string
	for _, device := range allDevices {
		if _, excluded := excludeDevices[device]; !excluded {
			filtered = append(filtered, device)
			r.logger.Debug(r.logTag, "Including device: %s", device)
		} else {
			r.logger.Debug(r.logTag, "Excluding device: %s (symlink: %s)", device, excludeDevices[device])
		}
	}
	return filtered
}
