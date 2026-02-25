package devicepathresolver

import (
	"sort"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type awsNVMeInstanceStorageResolver struct {
	fs                 boshsys.FileSystem
	devicePathResolver DevicePathResolver
	logger             boshlog.Logger
	logTag             string
	ebsSymlinkPattern  string
	nvmeDevicePattern  string
}

func NewAWSNVMeInstanceStorageResolver(
	fs boshsys.FileSystem,
	devicePathResolver DevicePathResolver,
	logger boshlog.Logger,
	ebsSymlinkPattern string,
	nvmeDevicePattern string,
) InstanceStorageResolver {
	if ebsSymlinkPattern == "" {
		ebsSymlinkPattern = "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_*"
	}
	if nvmeDevicePattern == "" {
		nvmeDevicePattern = "/dev/nvme*n1"
	}

	return &awsNVMeInstanceStorageResolver{
		fs:                 fs,
		devicePathResolver: devicePathResolver,
		logger:             logger,
		logTag:             "AWSNVMeInstanceStorageResolver",
		ebsSymlinkPattern:  ebsSymlinkPattern,
		nvmeDevicePattern:  nvmeDevicePattern,
	}
}

func (r *awsNVMeInstanceStorageResolver) DiscoverInstanceStorage(devices []boshsettings.DiskSettings) ([]string, error) {
	if len(devices) == 0 {
		return []string{}, nil
	}

	allNvmeDevices, err := r.fs.Glob(r.nvmeDevicePattern)
	if err != nil {
		return nil, bosherr.WrapError(err, "Globbing NVMe devices")
	}

	r.logger.Debug(r.logTag, "Found NVMe devices: %v", allNvmeDevices)

	ebsSymlinks, err := r.fs.Glob(r.ebsSymlinkPattern)
	if err != nil {
		return nil, bosherr.WrapError(err, "Globbing EBS symlinks")
	}

	ebsDevices := make(map[string]bool)
	for _, symlink := range ebsSymlinks {
		absPath, err := r.fs.ReadAndFollowLink(symlink)
		if err != nil {
			r.logger.Debug(r.logTag, "Could not resolve symlink %s: %s", symlink, err.Error())
			continue
		}

		r.logger.Debug(r.logTag, "EBS volume: %s -> %s", symlink, absPath)
		ebsDevices[absPath] = true
	}

	var instanceStorage []string
	for _, devicePath := range allNvmeDevices {
		if !ebsDevices[devicePath] {
			instanceStorage = append(instanceStorage, devicePath)
			r.logger.Info(r.logTag, "Discovered instance storage: %s", devicePath)
		} else {
			r.logger.Debug(r.logTag, "Excluding EBS volume: %s", devicePath)
		}
	}

	sort.Strings(instanceStorage)

	if len(instanceStorage) != len(devices) {
		return nil, bosherr.Errorf("Expected %d instance storage devices but discovered %d: %v",
			len(devices), len(instanceStorage), instanceStorage)
	}

	return instanceStorage, nil
}
