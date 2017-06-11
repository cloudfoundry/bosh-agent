package devicepathresolver

import (
	"fmt"
	"path"
	"strings"
	"time"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type mappedDevicePathResolver struct {
	diskWaitTimeout time.Duration
	fs              boshsys.FileSystem
	logger          boshlog.Logger
}

func NewMappedDevicePathResolver(
	diskWaitTimeout time.Duration,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) DevicePathResolver {
	return mappedDevicePathResolver{diskWaitTimeout, fs, logger}
}

func (dpr mappedDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	stopAfter := time.Now().Add(dpr.diskWaitTimeout)

	devicePath := diskSettings.Path
	if len(devicePath) == 0 {
		return "", false, bosherr.Error("Getting real device path: path is missing")
	}

	realPath, found := dpr.findPossibleDevice(devicePath)

	for !found {
		if time.Now().After(stopAfter) {
			return "", true, bosherr.Errorf("Timed out getting real device path for %s", devicePath)
		}

		time.Sleep(100 * time.Millisecond)

		realPath, found = dpr.findPossibleDevice(devicePath)
	}

	return realPath, false, nil
}

func (dpr mappedDevicePathResolver) findPossibleDevice(devicePath string) (string, bool) {
	err := dpr.scanForSCSIDevices()
	if err != nil {
		dpr.logger.Debug("mappedDevicePathResolver", "Failed scanning for SCSI devices", err)
	}

	needsMapping := strings.HasPrefix(devicePath, "/dev/sd")

	if needsMapping {
		pathSuffix := strings.Split(devicePath, "/dev/sd")[1]

		possiblePrefixes := []string{
			"/dev/xvd", // Xen
			"/dev/vd",  // KVM
			"/dev/sd",
		}

		for _, prefix := range possiblePrefixes {
			path := prefix + pathSuffix
			if dpr.fs.FileExists(path) {
				return path, true
			}
		}
	} else {
		if dpr.fs.FileExists(devicePath) {
			return devicePath, true
		}
	}

	return "", false
}

func (dpr mappedDevicePathResolver) scanForSCSIDevices() error {
	devicePaths, err := dpr.fs.Glob("/sys/bus/scsi/devices/*:0:0:0/block/*")
	if err != nil {
		return err
	}

	var hostID string

	for _, rootDevicePath := range devicePaths {
		if path.Base(rootDevicePath) == "sda" {
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
		return nil
	}

	scanPath := fmt.Sprintf("/sys/class/scsi_host/host%s/scan", hostID)

	return dpr.fs.WriteFileString(scanPath, "- - -")
}
