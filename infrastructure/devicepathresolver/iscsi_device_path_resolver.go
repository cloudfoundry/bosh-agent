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

const maxRecentDevices = 10

type recentDevice struct {
	path     string
	resolved time.Time
}

// iscsiDevicePathResolver resolves device path by performing Open-iscsi discovery
type iscsiDevicePathResolver struct {
	diskWaitTimeout time.Duration
	runner          boshsys.CmdRunner
	openiscsi       boshopeniscsi.OpenIscsi
	fs              boshsys.FileSystem
	dirProvider     boshdirs.Provider
	logTag          string
	logger          boshlog.Logger
	recentDevices   map[string]recentDevice
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
		recentDevices:   make(map[string]recentDevice),
	}
}

func (ispr iscsiDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	err := ispr.checkISCSISettings(diskSettings.ISCSISettings)
	if err != nil {
		return "", false, bosherr.WrapError(err, "Checking disk settings")
	}

	devicePath, found := ispr.getFromCache(diskSettings.ID)
	if found {
		ispr.logger.Info(ispr.logTag, "Found remembered path '%s'", devicePath)
		return devicePath, false, nil
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
		// KLUDGE (2021-12 review): why should we accept 2 but not 3 attached
		// (and partitioned) devices here? When migrating from an old disk to
		// a new one, we'll typically find the old disk here (we know it
		// because the disk ID is the lat mounted CID that has been written
		// on disk), and the new one after iSCSI discovery/logout/login in
		// getDevicePathAfterConnectTarget() below.
		//
		// Second concern is that Bosh allows many persistent disks to be
		// mounted. So, this limitation here seems inappropriate.
		// See also: https://www.starkandwayne.com/blog/bosh-multiple-disks/
		//
		// It really seems that the original implementer have made an invalid
		// assumption here.
		return "", false, bosherr.WrapError(err, "More than 2 persistent disks attached")
	}

	alreadySeen := lastDiskID == diskSettings.ID
	brandNew := lastDiskID == ""
	isPartitionned := len(existingPaths) > 0
	ispr.logger.Debug(ispr.logTag, "Found %d existing paths (alreadySeen:'%+v', brandNew:'%+v', isPartitionned:'%+v')", len(existingPaths), alreadySeen, brandNew, isPartitionned)
	if (alreadySeen || brandNew) && isPartitionned {
		// KLUDGE (2021-12 review): when facing 2 devices here, why would we
		// arbitrarily choose the first one. How do we know this is the right
		// one matching the disk ID? Practically, this algorithm seems to
		// work reliably only when mounting one new device at a time.
		ispr.logger.Info(ispr.logTag, "Found existing path '%s'", existingPaths[0])
		ispr.putToCache(diskSettings.ID, existingPaths[0])
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

	ispr.putToCache(diskSettings.ID, realPath)
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
func (ispr iscsiDevicePathResolver) getDevicePaths(devices []string, alreadyPartitioned bool) ([]string, error) {
	var partitionRegexp = regexp.MustCompile("-part1")
	var paths []string

	for _, device := range devices {
		fields := strings.Fields(device)
		if len(fields) == 0 {
			ispr.logger.Warn(ispr.logTag, "unexpected device in dmsetup output: '%+v'", device)
			continue
		}
		deviceName := fields[0]
		firstPartitionExists := partitionRegexp.MatchString(deviceName)
		if firstPartitionExists == alreadyPartitioned {
			matchedPath := path.Join("/dev/mapper", strings.Split(deviceName, "-")[0])
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

	hasBeenLoggedin, err := ispr.openiscsi.IsLoggedin()
	if err != nil {
		return bosherr.WrapError(err, "Could not check all sessions")
	}
	if hasBeenLoggedin {
		err = ispr.openiscsi.Logout()
		if err != nil {
			return bosherr.WrapError(err, "Could not logout all sessions")
		}
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

	if !ispr.fs.FileExists(managedDiskSettingsPath) {
		return "", nil
	}

	contents, err := ispr.fs.ReadFile(managedDiskSettingsPath)
	if err != nil {
		return "", bosherr.WrapError(err, "Reading managed_disk_settings.json")
	}

	return string(contents), nil
}

func (ispr iscsiDevicePathResolver) putToCache(diskCID, devicePath string) {
	if diskCID == "" {
		ispr.logger.Warn(ispr.logTag, "Cannot remember device path '%s' for empty disk CID", devicePath)
	}
	for len(ispr.recentDevices) >= maxRecentDevices {
		var (
			oldestCID    string
			oldestDevice recentDevice
		)
		// find the oldest entry to evict from cache
		for cid, device := range ispr.recentDevices {
			if oldestCID == "" || device.resolved.Before(oldestDevice.resolved) {
				oldestCID = cid
				oldestDevice = device
			}
		}
		ispr.logger.Warn(ispr.logTag, "Evicting from cache device ID '%s' with path '%s'", oldestCID, oldestDevice.path)
		delete(ispr.recentDevices, oldestCID)
	}
	ispr.logger.Warn(ispr.logTag, "Remembering device path '%s' for disk CID '%s'", devicePath, diskCID)
	ispr.recentDevices[diskCID] = recentDevice{
		path:     devicePath,
		resolved: time.Now(),
	}
}

func (ispr iscsiDevicePathResolver) getFromCache(diskCID string) (string, bool) {
	device, found := ispr.recentDevices[diskCID]
	if found {
		return device.path, found
	}
	return "", false
}
