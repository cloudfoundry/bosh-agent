package devicepathresolver

import (
	"sort"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

// Known symlink patterns for different cloud providers
const (
	// AWS EBS volumes create symlinks with this pattern
	AWSEBSSymlinkPattern = "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_*"
	// Default NVMe device pattern
	DefaultNVMeDevicePattern = "/dev/nvme*n1"
)

// nvmeSymlinkFilteringResolver discovers NVMe instance storage by filtering out
// IaaS-managed volumes identified via symlinks. This generic resolver works for
// any cloud provider that creates symlinks for their managed volumes.
type nvmeSymlinkFilteringResolver struct {
	fs                        boshsys.FileSystem
	logger                    boshlog.Logger
	logTag                    string
	managedDiskSymlinkPattern string
	nvmeDevicePattern         string
}

// NewNVMeSymlinkFilteringResolver creates a generic resolver that discovers
// instance storage by filtering out managed volumes identified via symlinks.
// This works for AWS (EBS).
func NewNVMeSymlinkFilteringResolver(
	fs boshsys.FileSystem,
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

	return &nvmeSymlinkFilteringResolver{
		fs:                        fs,
		logger:                    logger,
		logTag:                    "NVMeSymlinkFilteringResolver",
		managedDiskSymlinkPattern: managedDiskSymlinkPattern,
		nvmeDevicePattern:         nvmeDevicePattern,
	}
}

// NewAWSNVMeInstanceStorageResolver creates a resolver for AWS NVMe instances.
// Deprecated: Use NewNVMeSymlinkFilteringResolver with AWSEBSSymlinkPattern instead.
func NewAWSNVMeInstanceStorageResolver(
	fs boshsys.FileSystem,
	logger boshlog.Logger,
	ebsSymlinkPattern string,
	nvmeDevicePattern string,
) InstanceStorageResolver {
	if ebsSymlinkPattern == "" {
		ebsSymlinkPattern = AWSEBSSymlinkPattern
	}
	return NewNVMeSymlinkFilteringResolver(fs, logger, ebsSymlinkPattern, nvmeDevicePattern)
}

func (r *nvmeSymlinkFilteringResolver) DiscoverInstanceStorage(devices []boshsettings.DiskSettings) ([]string, error) {
	if len(devices) == 0 {
		return []string{}, nil
	}

	allNvmeDevices, err := r.fs.Glob(r.nvmeDevicePattern)
	if err != nil {
		return nil, bosherr.WrapError(err, "Globbing NVMe devices")
	}

	r.logger.Debug(r.logTag, "Found NVMe devices: %v", allNvmeDevices)

	managedDiskSymlinks, err := r.fs.Glob(r.managedDiskSymlinkPattern)
	if err != nil {
		return nil, bosherr.WrapError(err, "Globbing managed disk symlinks")
	}

	// Build a map of managed disk device paths to exclude
	managedDevices := make(map[string]bool)
	for _, symlink := range managedDiskSymlinks {
		absPath, err := r.fs.ReadAndFollowLink(symlink)
		if err != nil {
			r.logger.Debug(r.logTag, "Could not resolve symlink %s: %s", symlink, err.Error())
			continue
		}

		r.logger.Debug(r.logTag, "Managed disk: %s -> %s", symlink, absPath)
		managedDevices[absPath] = true
	}

	// Instance storage = all NVMe devices minus managed disks
	var instanceStorage []string
	for _, devicePath := range allNvmeDevices {
		if !managedDevices[devicePath] {
			instanceStorage = append(instanceStorage, devicePath)
			r.logger.Info(r.logTag, "Discovered instance storage: %s", devicePath)
		} else {
			r.logger.Debug(r.logTag, "Excluding managed disk: %s", devicePath)
		}
	}

	sort.Strings(instanceStorage)

	if len(instanceStorage) != len(devices) {
		return nil, bosherr.Errorf("Expected %d instance storage devices but discovered %d: %v",
			len(devices), len(instanceStorage), instanceStorage)
	}

	return instanceStorage, nil
}
