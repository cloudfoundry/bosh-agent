package devicepathresolver

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	boshopeniscsi "github.com/cloudfoundry/bosh-agent/platform/openiscsi"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
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
	err := ispr.checkISCSISettings(diskSettings.ISCSISettings)
	if err != nil {
		return "", false, bosherr.WrapError(err, "Checking disk settings")
	}

	// fetch existing path if disk is last mounted
	lastDiskID, err := ispr.lastMountedCid()
	if err != nil {
		return "", false, bosherr.WrapError(err, "Fetching last mounted disk CID")
	}

	ispr.logger.Debug(ispr.logTag, "Last mounted disk CID: '%s'", lastDiskID)

	mappedDevices, err := ispr.getMappedDevices()
	if err != nil {
		return "", false, bosherr.WrapError(err, "Getting mapped devices")
	}

	var existingPaths []string
	existingPaths, err = ispr.getDevicePaths(mappedDevices, true)
	if err != nil {
		return "", false, bosherr.WrapError(err, "Getting existing paths")
	}

	ispr.logger.Debug(ispr.logTag, "Existing real paths '%+v'", existingPaths)
	if len(existingPaths) > 2 {
		return "", false, bosherr.WrapError(err, "More than 2 persistent disks attached")
	}

	if lastDiskID == diskSettings.ID && len(existingPaths) > 0 {
		ispr.logger.Info(ispr.logTag, "Found existing path '%s'", existingPaths[0])
		return existingPaths[0], false, nil
	}

	err = ispr.connectTarget(diskSettings.ISCSISettings)
	if err != nil {
		return "", false, bosherr.WrapError(err, "connecting iSCSI target")
	}

	// Combine paths to whole string to filter target path
	exstingPathString := strings.Join(existingPaths, ",")
	realPath, err := ispr.getDevicePathAfterConnectTarget(exstingPathString)
	if err != nil {
		if strings.Contains(err.Error(), "Timed out to get real iSCSI device path") {
			return "", true, bosherr.WrapError(err, "get device path after connect iSCSI target")
		}
		return "", false, bosherr.WrapError(err, "get device path after connect iSCSI target")
	}

	return realPath, false, nil
}

func (ispr iscsiDevicePathResolver) checkISCSISettings(iSCSISettings boshsettings.ISCSISettings) error {
	if iSCSISettings.InitiatorName == "" {
		return bosherr.Errorf("iSCSI InitiatorName is not set")
	}

	if iSCSISettings.Username == "" {
		return bosherr.Errorf("iSCSI Username is not set")
	}

	if iSCSISettings.Password == "" {
		return bosherr.Errorf("iSCSI Password is not set")
	}

	if iSCSISettings.Target == "" {
		return bosherr.Errorf("iSCSI Target is not set")
	}
	return nil
}

func (ispr iscsiDevicePathResolver) getMappedDevices() ([]string, error) {
	var devices []string
	result, _, _, err := ispr.runner.RunCommand("dmsetup", "ls")
	if err != nil {
		return devices, bosherr.WrapError(err, "listing mapped devices")
	}

	if strings.Contains(result, "No devices found") {
		return devices, nil
	}

	devices = strings.Split(strings.Trim(result, "\n"), "\n")
	ispr.logger.Debug(ispr.logTag, "devices: '%+v'", devices)

	return devices, nil
}

// getDevicePaths: to find iSCSI device paths
// a "–part1" suffix device based on origin multipath device
// last mounted disk already have this device, new disk doesn't have this device yet
func (ispr iscsiDevicePathResolver) getDevicePaths(devices []string, shouldExist bool) ([]string, error) {
	var paths []string
	for _, device := range devices {
		exist, err := regexp.MatchString("-part1", device)
		if err != nil {
			return paths, bosherr.WrapError(err, "There is a problem with your regexp: '-part1'. That is used to find existing device")
		}
		if exist == shouldExist {
			matchedPath := path.Join("/dev/mapper", strings.Split(strings.Fields(device)[0], "-")[0])
			ispr.logger.Debug(ispr.logTag, "path in device list: '%+v'", matchedPath)
			paths = append(paths, matchedPath)
		}
	}

	return paths, nil
}

func (ispr iscsiDevicePathResolver) connectTarget(iSCSISettings boshsettings.ISCSISettings) error {
	err := ispr.openiscsi.Setup(iSCSISettings.InitiatorName, iSCSISettings.Username, iSCSISettings.Password)
	if err != nil {
		return bosherr.WrapError(err, "Could not setup Open-iSCSI")
	}

	err = ispr.openiscsi.Discovery(iSCSISettings.Target)
	if err != nil {
		return bosherr.WrapError(err, fmt.Sprintf("Could not discovery lun against portal %s", iSCSISettings.Target))
	}

	err = ispr.openiscsi.Login()
	if err != nil {
		return bosherr.WrapError(err, "Could not login all sessions")
	}

	return nil
}

func (ispr iscsiDevicePathResolver) getDevicePathAfterConnectTarget(existingPath string) (string, error) {
	ispr.logger.Debug(ispr.logTag, "Waiting for iSCSI device to appear")

	timer := time.NewTimer(ispr.diskWaitTimeout)

	for {
		select {
		case <-timer.C:
			return "", bosherr.Errorf("Timed out to get real iSCSI device path")
		default:
			mappedDevices, err := ispr.getMappedDevices()
			if err != nil {
				return "", bosherr.WrapError(err, "Getting mapped devices")
			}

			var realPaths []string
			realPaths, err = ispr.getDevicePaths(mappedDevices, false)
			if err != nil {
				return "", bosherr.WrapError(err, "Getting real paths")
			}

			if existingPath == "" && len(realPaths) == 1 {
				ispr.logger.Info(ispr.logTag, "Found real path '%s'", realPaths[0])
				return realPaths[0], nil
			}

			for _, realPath := range realPaths {
				if strings.Contains(existingPath, realPath) {
					continue
				} else {
					ispr.logger.Info(ispr.logTag, "Found real path '%s'", realPath)
					return realPath, nil
				}
			}
		}

		time.Sleep(5 * time.Second)
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
