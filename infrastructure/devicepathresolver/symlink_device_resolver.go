package devicepathresolver

import (
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

// Cloud provider symlink patterns for managed volume identification.
// These patterns identify IaaS-managed volumes (EBS, Azure Managed Disks, etc.)
// that should be excluded when discovering instance/ephemeral storage.
const (
	// AWSEBSSymlinkPattern identifies AWS EBS volumes via NVMe symlinks.
	// EBS volumes on Nitro instances appear as NVMe devices with these symlinks.
	AWSEBSSymlinkPattern = "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_*"

	// AzureManagedDiskSymlinkPattern identifies Azure managed disks via LUN symlinks.
	AzureManagedDiskSymlinkPattern = "/dev/disk/azure/scsi1/lun*"

	// GCPPersistentDiskSymlinkPattern identifies GCP persistent disks.
	// GCP uses google-* symlinks for attached persistent disks.
	GCPPersistentDiskSymlinkPattern = "/dev/disk/by-id/google-*"
)

// Cloud provider symlink base paths for LUN-based device resolution.
const (
	// AzureLunSymlinkBasePath is the base path for Azure LUN symlinks.
	// Used by SymlinkLunDevicePathResolver for Azure NVMe disk resolution.
	AzureLunSymlinkBasePath = "/dev/disk/azure/data/by-lun"
)

// Default device patterns for NVMe instance storage discovery.
const (
	// DefaultNVMeDevicePattern matches NVMe namespace devices.
	DefaultNVMeDevicePattern = "/dev/nvme*n1"
)

// SymlinkDeviceResolver provides common symlink resolution functionality
// used by both AWS (for filtering out EBS) and Azure (for finding LUN devices).
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

// WaitForSymlink waits for a symlink to appear and resolves it to a device path.
// This is useful for Azure LUN resolution where disks may not be immediately available.
func (r *SymlinkDeviceResolver) WaitForSymlink(symlinkPath string, timeout time.Duration) (string, error) {
	stopAfter := time.Now().Add(timeout)

	for {
		if time.Now().After(stopAfter) {
			return "", bosherr.Errorf("Timed out waiting for symlink '%s' to resolve", symlinkPath)
		}

		realPath, err := r.fs.ReadAndFollowLink(symlinkPath)
		if err != nil {
			r.logger.Debug(r.logTag, "Symlink '%s' not yet available: %s", symlinkPath, err.Error())
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if r.fs.FileExists(realPath) {
			r.logger.Debug(r.logTag, "Resolved symlink '%s' to real path '%s'", symlinkPath, realPath)
			return realPath, nil
		}

		r.logger.Debug(r.logTag, "Real path '%s' does not yet exist", realPath)
		time.Sleep(100 * time.Millisecond)
	}
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
