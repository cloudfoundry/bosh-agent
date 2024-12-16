package devicepathresolver

import (
	"fmt"
	"path"
	"regexp"
	"time"

	boshudev "github.com/cloudfoundry/bosh-agent/v2/platform/udevdevice"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type idDevicePathResolver struct {
	diskWaitTimeout     time.Duration
	udev                boshudev.UdevDevice
	fs                  boshsys.FileSystem
	stripVolumeRegex    string
	stripVolumeCompiled *regexp.Regexp
}

func NewIDDevicePathResolver(
	diskWaitTimeout time.Duration,
	udev boshudev.UdevDevice,
	fs boshsys.FileSystem,
	stripVolumeRegex string,
) DevicePathResolver {
	return &idDevicePathResolver{
		diskWaitTimeout:     diskWaitTimeout,
		udev:                udev,
		fs:                  fs,
		stripVolumeRegex:    stripVolumeRegex,
		stripVolumeCompiled: nil,
	}
}

func (idpr *idDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	if diskSettings.ID == "" {
		return "", false, bosherr.Errorf("Disk ID is not set")
	}

	if len(diskSettings.ID) < 20 {
		return "", false, bosherr.Errorf("Disk ID is not the correct format")
	}

	err := idpr.udev.Trigger()
	if err != nil {
		return "", false, bosherr.WrapError(err, "Running udevadm trigger")
	}

	err = idpr.udev.Settle()
	if err != nil {
		return "", false, bosherr.WrapError(err, "Running udevadm settle")
	}

	stopAfter := time.Now().Add(idpr.diskWaitTimeout)
	found := false

	var realPath string

	diskID := diskSettings.ID
	strippedDiskID, err := idpr.stripVolumeIfRequired(diskID)
	if err != nil {
		return "", false, err
	}

	deviceGlobPattern := fmt.Sprintf("*%s", strippedDiskID)
	deviceIDPathGlobPattern := path.Join("/", "dev", "disk", "by-id", deviceGlobPattern)

	for !found {
		if time.Now().After(stopAfter) {
			return "", true, bosherr.Errorf("Timed out getting real device path for '%s'", diskID)
		}

		time.Sleep(100 * time.Millisecond)
		pathMatches, err := idpr.fs.Glob(deviceIDPathGlobPattern)
		if err != nil {
			continue
		}

		switch len(pathMatches) {
		case 0:
			continue
		case 1:
			realPath, err = idpr.fs.ReadAndFollowLink(pathMatches[0])
			if err != nil {
				continue
			}

			if idpr.fs.FileExists(realPath) {
				found = true
			}
		default:
			return "", true, bosherr.Errorf("More than one disk matched glob %q while getting real device path for %q", deviceIDPathGlobPattern, diskID)
		}
	}

	return realPath, false, nil
}

func (idpr *idDevicePathResolver) stripVolumeIfRequired(diskID string) (string, error) {
	if idpr.stripVolumeRegex == "" {
		return diskID, nil
	}

	if idpr.stripVolumeCompiled == nil {
		var err error
		idpr.stripVolumeCompiled, err = regexp.Compile(idpr.stripVolumeRegex)
		if err != nil {
			return "", bosherr.WrapError(err, "Compiling stripVolumeRegex")
		}
	}
	return idpr.stripVolumeCompiled.ReplaceAllLiteralString(diskID, ""), nil
}
