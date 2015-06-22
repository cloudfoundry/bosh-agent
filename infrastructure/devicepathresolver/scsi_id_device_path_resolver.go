package devicepathresolver

import (
	"regexp"
	"strings"
	"time"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

// Resolves device path by performing a SCSI rescan then looking under
// "/dev/disk/by-id/*uuid" where "uuid" is the cloud ID of the disk
type scsiIDDevicePathResolver struct {
	diskWaitTimeout time.Duration
	fs              boshsys.FileSystem
	logger          boshlog.Logger
}

func NewScsiIDDevicePathResolver(
	diskWaitTimeout time.Duration,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) DevicePathResolver {
	return scsiIDDevicePathResolver{
		diskWaitTimeout: diskWaitTimeout,
		fs:              fs,
		logger:          logger,
	}
}

const logTag = "scsiIDresolver"

func (idpr scsiIDDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	if diskSettings.ID == "" {
		err := bosherr.Errorf("Disk ID is not set")
		idpr.logger.Error(logTag, err.Error())
		return "", false, err
	}

	pattern := `^\w{8}(-\w{4}){3}-\w{12}$`
	match, _ := regexp.MatchString(pattern, diskSettings.ID)

	if !match {
		err := bosherr.Errorf("Disk ID is not a UUID")
		idpr.logger.Error(logTag, err.Error())
		return "", false, err
	}

	hostPaths, err := idpr.fs.Glob("/sys/class/scsi_host/host*/scan")
	if err != nil {
		err := bosherr.WrapError(err, "Could not list SCSI hosts")
		idpr.logger.Error(logTag, err.Error())
		return "", false, err
	}

	for _, hostPath := range hostPaths {
		idpr.logger.Info(logTag, "Performing SCSI rescan of "+hostPath)
		err = idpr.fs.WriteFileString(hostPath, "- - -")
		if err != nil {
			err := bosherr.WrapError(err, "Starting SCSI rescan")
			idpr.logger.Error(logTag, err.Error())
			return "", false, err
		}
	}

	stopAfter := time.Now().Add(idpr.diskWaitTimeout)
	found := false

	var realPath string

	for !found {
		idpr.logger.Debug(logTag, "Waiting for device to appear")

		if time.Now().After(stopAfter) {
			err := bosherr.Errorf("Timed out getting real device path for '%s'", diskSettings.ID)
			idpr.logger.Error(logTag, err.Error())
			return "", true, err
		}

		time.Sleep(100 * time.Millisecond)

		uuid := strings.Replace(diskSettings.ID, "-", "", -1)
		disks, err := idpr.fs.Glob("/dev/disk/by-id/*" + uuid)
		if err != nil {
			err := bosherr.WrapError(err, "Could not list disks by id")
			idpr.logger.Error(logTag, err.Error())
			return "", false, err
		}
		for _, path := range disks {
			idpr.logger.Debug(logTag, "Reading link "+path)
			realPath, err = idpr.fs.ReadLink(path)
			if err != nil {
				continue
			}

			if idpr.fs.FileExists(realPath) {
				idpr.logger.Debug(logTag, "Found real path "+realPath)
				found = true
				break
			}
		}
	}

	return realPath, false, nil
}
