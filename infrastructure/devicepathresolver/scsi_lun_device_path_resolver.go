package devicepathresolver

import (
	"fmt"
	"path"
	"time"
	"strings"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type SCSILunDevicePathResolver struct {
	diskWaitTimeout time.Duration
	fs              boshsys.FileSystem

	logTag string
	logger boshlog.Logger
}

func NewSCSILunDevicePathResolver(
	diskWaitTimeout time.Duration,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) SCSILunDevicePathResolver {
	return SCSILunDevicePathResolver{
		fs:              fs,
		diskWaitTimeout: diskWaitTimeout,

		logTag: "scsiLunResolver",
		logger: logger,
	}
}

func (ldpr SCSILunDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	if diskSettings.Lun == "" {
		return "", false, bosherr.Error("Disk lun is not set")
	}
	if diskSettings.HostDeviceID == "" {
		return "", false, bosherr.Error("Disk host_device_id is not set")
	}

	hostPaths, err := ldpr.fs.Glob("/sys/class/scsi_host/host*/scan")
	if err != nil {
		return "", false, bosherr.WrapError(err, "Could not list SCSI hosts")
	}

	for _, hostPath := range hostPaths {
		ldpr.logger.Info(ldpr.logTag, "Performing SCSI rescan of %s", hostPath)
		err = ldpr.fs.WriteFileString(hostPath, "- - -")
		if err != nil {
			return "", false, bosherr.WrapError(err, "Starting SCSI rescan")
		}
	}

	stopAfter := time.Now().Add(ldpr.diskWaitTimeout)

	var vmBusDeviceForDataDisks string

	vmBusDevices, err := ldpr.fs.Glob("/sys/bus/vmbus/devices/*/device_id")
	if err != nil {
		return "", false, bosherr.WrapError(err, "Could not list vmbus devices")
	}

	for _, vmBusDevice := range vmBusDevices {
		deviceID, err := ldpr.fs.ReadFileString(vmBusDevice)
		if err != nil {
			continue
		}
		if strings.Compare(strings.TrimSpace(deviceID), diskSettings.HostDeviceID) == 0 {
			vmBusDeviceSplits := strings.Split(vmBusDevice, "/")
			vmBusDeviceForDataDisks = vmBusDeviceSplits[5]
			break
		}
	}

	if vmBusDeviceForDataDisks == "" {
		return "", false, bosherr.WrapErrorf(err, "Cannot find the vmbus device by host_device_id '%s'", diskSettings.HostDeviceID)
	}
	ldpr.logger.Debug(ldpr.logTag, "Find the vmbus device '%s' by host_device_id '%s'", vmBusDeviceForDataDisks, diskSettings.HostDeviceID)

	deviceGlobPath := fmt.Sprintf("/sys/bus/scsi/devices/*:*:*:%s/block/*", diskSettings.Lun)
	found := false
	var realPath string
	
	for !found {
		ldpr.logger.Debug(ldpr.logTag, "Waiting for device to appear")

		if time.Now().After(stopAfter) {
			return "", true, bosherr.Errorf("Timed out getting real device path by lun '%s' and host_device_id '%s'", diskSettings.Lun, diskSettings.HostDeviceID)
		}

		time.Sleep(100 * time.Millisecond)

		devicePaths, err := ldpr.fs.Glob(deviceGlobPath)
		if err != nil {
			return "", false, bosherr.WrapErrorf(err, "Could not list disks by lun '%s'", diskSettings.Lun)
		}

		for _, devicePath := range devicePaths {
			basename := path.Base(devicePath)
			tempPath, err := ldpr.fs.ReadLink(path.Join("/sys/class/block/", basename))
			if err != nil {
				continue
			}

			if strings.Contains(tempPath, "/" + vmBusDeviceForDataDisks + "/") {
				realPath = path.Join("/dev/", basename)
				if ldpr.fs.FileExists(realPath) {
					ldpr.logger.Debug(ldpr.logTag, "Found real path " + realPath)
					found = true
					break
				}
			}
		}
	}

	return realPath, false, nil
}