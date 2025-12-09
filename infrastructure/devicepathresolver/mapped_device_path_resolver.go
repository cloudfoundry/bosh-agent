package devicepathresolver

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type mappedDevicePathResolver struct {
	diskWaitTimeout time.Duration
	fs              boshsys.FileSystem
}

func NewMappedDevicePathResolver(
	diskWaitTimeout time.Duration,
	fs boshsys.FileSystem,
) DevicePathResolver {
	return mappedDevicePathResolver{fs: fs, diskWaitTimeout: diskWaitTimeout}
}

func (dpr mappedDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	stopAfter := time.Now().Add(dpr.diskWaitTimeout)

	devicePath := diskSettings.Path
	if len(devicePath) == 0 {
		return "", false, bosherr.Error("Getting real device path: path is missing")
	}

	realPath, found, err := dpr.findPossibleDevice(devicePath)
	if err != nil {
		return "", false, bosherr.Errorf("Getting real device path: %s", err.Error())
	}

	for !found {
		if time.Now().After(stopAfter) {
			return "", true, bosherr.Errorf("Timed out getting real device path for %s", devicePath)
		}

		time.Sleep(100 * time.Millisecond)

		realPath, found, err = dpr.findPossibleDevice(devicePath)
		if err != nil {
			return "", false, bosherr.Errorf("Getting real device path: %s", err.Error())
		}
	}

	return realPath, false, nil
}

func (dpr mappedDevicePathResolver) findPossibleDevice(devicePath string) (string, bool, error) {
	needsMapping := strings.HasPrefix(devicePath, "/dev/sd")

	if needsMapping { //nolint:nestif
		pathSuffix := strings.Split(devicePath, "/dev/sd")[1]

		possiblePrefixes := []string{
			"/dev/xvd", // Xen
			"/dev/vd",  // KVM
			"/dev/sd",
		}

		for _, prefix := range possiblePrefixes {
			path := prefix + pathSuffix
			if dpr.fs.FileExists(path) {
				return path, true, nil
			}
		}
	} else if dpr.fs.FileExists(devicePath) {
		stat, err := dpr.fs.Lstat(devicePath)
		if err == nil && stat.Mode()&os.ModeSymlink == os.ModeSymlink {
			link, err := dpr.fs.Readlink(devicePath)
			if err != nil {
				return "", false, err
			}

			if strings.Contains(link, "..") {
				link = filepath.Join(devicePath, "..", link)
			}

			return filepath.Clean(link), true, nil
		}

		return devicePath, true, nil
	}

	return "", false, nil
}
