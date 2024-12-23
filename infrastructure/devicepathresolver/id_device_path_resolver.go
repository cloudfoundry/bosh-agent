package devicepathresolver

import (
	"fmt"
	"path"
	"regexp"
	"time"

	boshudev "github.com/cloudfoundry/bosh-agent/v2/platform/udevdevice"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type idDevicePathResolver struct {
	diskWaitTimeout            time.Duration
	udev                       boshudev.UdevDevice
	fs                         boshsys.FileSystem
	DiskIDTransformPattern     string
	DiskIDTransformReplacement string
	logTag                     string
	logger                     boshlog.Logger
}

func NewIDDevicePathResolver(
	diskWaitTimeout time.Duration,
	udev boshudev.UdevDevice,
	fs boshsys.FileSystem,
	DiskIDTransformPattern string,
	DiskIDTransformReplacement string,
	logger boshlog.Logger,
) DevicePathResolver {
	return &idDevicePathResolver{
		diskWaitTimeout:            diskWaitTimeout,
		udev:                       udev,
		fs:                         fs,
		DiskIDTransformPattern:     DiskIDTransformPattern,
		DiskIDTransformReplacement: DiskIDTransformReplacement,
		logTag:                     "IdDevicePathResolver",
		logger:                     logger,
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
	TransformedDiskID, err := idpr.TransformDiskID(diskID)
	if err != nil {
		return "", false, err
	}

	deviceGlobPattern := TransformedDiskID
	if idpr.DiskIDTransformPattern == "" {
		deviceGlobPattern = fmt.Sprintf("*%s", TransformedDiskID)
	}

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

func (idpr *idDevicePathResolver) TransformDiskID(diskID string) (string, error) {
	if idpr.DiskIDTransformPattern == "" {
		idpr.logger.Debug(idpr.logTag, "DiskIDTransformRules is empty, returning diskID as is")
		return diskID, nil
	}

	transformed := diskID
	re := regexp.MustCompile(idpr.DiskIDTransformPattern)
	if re.MatchString(transformed) {
		transformed = re.ReplaceAllString(transformed, idpr.DiskIDTransformReplacement)
		idpr.logger.Debug(idpr.logTag, "DiskIDTransformRules: Original: '%+v' -> Transformed: '%+v'\n", diskID, transformed)
	}
	return transformed, nil
}
