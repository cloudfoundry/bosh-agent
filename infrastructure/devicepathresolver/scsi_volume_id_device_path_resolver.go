package devicepathresolver

import (
	"fmt"
	"path"
	"strings"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

const maxScanRetries = 30

type SCSIVolumeIDDevicePathResolver struct {
	diskWaitTimeout time.Duration
	fs              boshsys.FileSystem
}

func NewSCSIVolumeIDDevicePathResolver(
	diskWaitTimeout time.Duration,
	fs boshsys.FileSystem,
) SCSIVolumeIDDevicePathResolver {
	return SCSIVolumeIDDevicePathResolver{
		fs:              fs,
		diskWaitTimeout: diskWaitTimeout,
	}
}

func (devicePathResolver SCSIVolumeIDDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	devicePaths, err := devicePathResolver.fs.Glob("/sys/bus/scsi/devices/*:0:0:0/block/*")
	if err != nil {
		return "", false, err
	}

	var hostID string

	volumeID := diskSettings.VolumeID

	for _, rootDevicePath := range devicePaths {
		if strings.HasPrefix(path.Base(rootDevicePath), "sd") {
			rootDevicePathSplits := strings.Split(rootDevicePath, "/")
			if len(rootDevicePathSplits) > 5 {
				scsiPath := rootDevicePathSplits[5]
				scsiPathSplits := strings.Split(scsiPath, ":")
				if len(scsiPathSplits) > 0 {
					hostID = scsiPathSplits[0]
				}
			}
		}
	}

	if len(hostID) == 0 {
		return "", false, bosherr.Error("Zero length hostID")
	}

	scanPath := fmt.Sprintf("/sys/class/scsi_host/host%s/scan", hostID)
	err = devicePathResolver.fs.WriteFileString(scanPath, "- - -")
	if err != nil {
		return "", false, err
	}

	deviceGlobPath := fmt.Sprintf("/sys/bus/scsi/devices/%s:0:%s:0/block/*", hostID, volumeID)

	for i := 0; i < maxScanRetries; i++ {
		devicePaths, err = devicePathResolver.fs.Glob(deviceGlobPath)
		if err != nil || len(devicePaths) == 0 {
			time.Sleep(devicePathResolver.diskWaitTimeout)
			continue
		} else {
			break
		}
	}

	if err != nil {
		return "", false, err
	}

	if len(devicePaths) == 0 {
		return "", false, nil // see infrastructure/devicepathresolver/scsi_volume_id_device_path_resolver_test.go:102
	}

	basename := path.Base(devicePaths[0])
	realPath := path.Join("/dev/", basename)

	return realPath, false, nil
}
