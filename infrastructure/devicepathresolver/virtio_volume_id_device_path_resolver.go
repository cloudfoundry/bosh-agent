package devicepathresolver

import (
	"fmt"
	"path"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshudev "github.com/cloudfoundry/bosh-agent/v2/platform/udevdevice"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type virtioVolumeIDDevicePathResolver struct {
	diskWaitTimeout time.Duration
	udev            boshudev.UdevDevice
	fs              boshsys.FileSystem
	logger          boshlog.Logger
	logTag          string
}

func NewVirtioVolumeIDDevicePathResolver(
	diskWaitTimeout time.Duration,
	udev boshudev.UdevDevice,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) DevicePathResolver {
	return &virtioVolumeIDDevicePathResolver{
		diskWaitTimeout: diskWaitTimeout,
		udev:            udev,
		fs:              fs,
		logger:          logger,
		logTag:          "VirtioVolumeIDDevicePathResolver",
	}
}

func (vpr *virtioVolumeIDDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	if diskSettings.VolumeID == "" {
		return "", false, bosherr.Errorf("Disk VolumeID is not set")
	}

	if len(diskSettings.VolumeID) < 20 {
		return "", false, bosherr.Errorf("Disk VolumeID is not the correct format")
	}

	err := vpr.udev.Trigger()
	if err != nil {
		return "", false, bosherr.WrapError(err, "Running udevadm trigger")
	}

	err = vpr.udev.Settle()
	if err != nil {
		return "", false, bosherr.WrapError(err, "Running udevadm settle")
	}

	stopAfter := time.Now().Add(vpr.diskWaitTimeout)
	found := false

	var realPath string

	volumeID := diskSettings.VolumeID

	deviceGlobPattern := fmt.Sprintf("*%s*", volumeID)
	deviceIDPathGlobPattern := path.Join("/", "dev", "disk", "by-id", deviceGlobPattern)

	vpr.logger.Debug(vpr.logTag, "Searching for device with VolumeID '%s' using pattern '%s'", volumeID, deviceIDPathGlobPattern)

	for !found {
		if time.Now().After(stopAfter) {
			return "", true, bosherr.Errorf("Timed out getting real device path for VolumeID '%s'", volumeID)
		}

		time.Sleep(100 * time.Millisecond)
		pathMatches, err := vpr.fs.Glob(deviceIDPathGlobPattern)
		if err != nil {
			continue
		}

		switch len(pathMatches) {
		case 0:
			continue
		case 1:
			realPath, err = vpr.fs.ReadAndFollowLink(pathMatches[0])
			if err != nil {
				continue
			}

			if vpr.fs.FileExists(realPath) {
				found = true
				vpr.logger.Debug(vpr.logTag, "Found device for VolumeID '%s' at '%s'", volumeID, realPath)
			}
		default:
			return "", true, bosherr.Errorf("More than one disk matched glob %q while getting real device path for VolumeID %q", deviceIDPathGlobPattern, volumeID)
		}
	}

	return realPath, false, nil
}
