package devicepathresolver

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	boshopeniscsi "github.com/cloudfoundry/bosh-agent/platform/openiscsi"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"path"
	"path/filepath"
)

// iscsiDevicePathResolver resolves device path by performing Open-iscsi discovery
type iscsiDevicePathResolver struct {
	diskWaitTimeout time.Duration
	runner          boshsys.CmdRunner
	openiscsi       boshopeniscsi.OpenIscsi
	fs              boshsys.FileSystem
	dirProvider     boshdirs.Provider
	logTag          string
	logger          boshlog.Logger
}

func NewIscsiDevicePathResolver(
	diskWaitTimeout time.Duration,
	runner boshsys.CmdRunner,
	openiscsi boshopeniscsi.OpenIscsi,
	fs boshsys.FileSystem,
	dirProvider boshdirs.Provider,
	logger boshlog.Logger,
) DevicePathResolver {
	return iscsiDevicePathResolver{
		diskWaitTimeout: diskWaitTimeout,
		runner:          runner,
		openiscsi:       openiscsi,
		fs:              fs,
		dirProvider:     dirProvider,
		logTag:          "iscsiResolver",
		logger:          logger,
	}
}

func (ispr iscsiDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	var lastDiskID string

	if diskSettings.InitiatorName == "" {
		return "", false, bosherr.Errorf("ISCSI InitiatorName is not set")
	}

	if diskSettings.Username == "" {
		return "", false, bosherr.Errorf("ISCSI Username is not set")
	}

	if diskSettings.Password == "" {
		return "", false, bosherr.Errorf("ISCSI Password is not set")
	}

	if diskSettings.Target == "" {
		return "", false, bosherr.Errorf("ISCSI Iface Ipaddress is not set")
	}

	existingPaths := []string{}

	result, _, _, err := ispr.runner.RunCommand("dmsetup", "ls")
	if err != nil {
		return "", false, bosherr.WrapError(err, "Could not determining device mapper entries")
	}

	lastDiskID, err = ispr.lastMountedCid()
	if err != nil {
		return "", false, bosherr.WrapError(err, "Fetching last mounted disk CID")
	}

	ispr.logger.Debug(ispr.logTag, "Last mounted disk CID: '%s'", lastDiskID)

	if !strings.Contains(result, "No devices found") {
		lines := strings.Split(strings.Trim(result, "\n"), "\n")
		ispr.logger.Debug(ispr.logTag, "lines: '%+v'", lines)
		for _, line := range lines {
			if match, _ := regexp.MatchString("-part1", line); match {
				exitingPath := path.Join("/dev/mapper", strings.Split(strings.Fields(line)[0], "-")[0])
				ispr.logger.Debug(ispr.logTag, "exitingPath in lines: '%+v'", exitingPath)
				if lastDiskID == diskSettings.ID {
					return exitingPath, false, nil
				}
				existingPaths = append(existingPaths, exitingPath)
			}
		}
	}

	ispr.logger.Debug(ispr.logTag, "Existing real paths '%+v'", existingPaths)
	if len(existingPaths) > 2 {
		return "", false, bosherr.WrapError(err, "More than 2 persistent disks attached")
	}

	err = ispr.openiscsi.Setup(diskSettings.InitiatorName, diskSettings.Username, diskSettings.Password)
	if err != nil {
		return "", false, bosherr.WrapError(err, "Could not setup Open-iscsi")
	}

	err = ispr.openiscsi.Discovery(diskSettings.Target)
	if err != nil {
		return "", false, bosherr.WrapError(err, fmt.Sprintf("Could not discovery lun against portal %s", diskSettings.Target))
	}

	err = ispr.openiscsi.Login()
	if err != nil {
		return "", false, bosherr.WrapError(err, "Could not login all sessions")
	}

	stopAfter := time.Now().Add(ispr.diskWaitTimeout)

	for {
		ispr.logger.Debug(ispr.logTag, "Waiting for device to appear")

		if time.Now().After(stopAfter) {
			return "", true, bosherr.Errorf("Timed out getting real device path by portal '%s'", diskSettings.Target)
		}

		time.Sleep(5 * time.Second)

		result, _, _, err := ispr.runner.RunCommand("dmsetup", "ls")
		if err != nil {
			return "", false, bosherr.WrapError(err, "Could not determining device mapper entries")
		}

		if strings.Contains(result, "No devices found") {
			continue
		}

		lines := strings.Split(strings.Trim(result, "\n"), "\n")
		for _, line := range lines {
			if match, _ := regexp.MatchString("-part1", line); !match {
				matchedPath := path.Join("/dev/mapper", strings.Fields(line)[0])

				if len(existingPaths) == 0 {
					ispr.logger.Debug(ispr.logTag, "Found real path '%s'", matchedPath)
					return matchedPath, false, nil
				}

				for _, existingPath := range existingPaths {
					if matchedPath == existingPath {
						continue
					} else {
						ispr.logger.Debug(ispr.logTag, "Found real path '%s'", matchedPath)
						return matchedPath, false, nil
					}
				}
			}
		}
	}
}

func (ispr iscsiDevicePathResolver) lastMountedCid() (string, error) {
	managedDiskSettingsPath := filepath.Join(ispr.dirProvider.BoshDir(), "managed_disk_settings.json")
	var lastMountedCid string

	if ispr.fs.FileExists(managedDiskSettingsPath) {
		contents, err := ispr.fs.ReadFile(managedDiskSettingsPath)
		if err != nil {
			return "", bosherr.WrapError(err, "Reading managed_disk_settings.json")
		}
		lastMountedCid = string(contents)

		return lastMountedCid, nil
	}

	return "", nil
}
