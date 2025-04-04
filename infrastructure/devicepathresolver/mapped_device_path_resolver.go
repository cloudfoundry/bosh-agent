package devicepathresolver

import (
	"fmt"
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
		pathLetterSuffix := strings.Split(devicePath, "/dev/sd")[1]
		pathNumberSuffix := fmt.Sprintf("%d", pathLetterSuffix[0]-97) // a=0, b=1

		possiblePaths := []string{
			"/dev/xvd" + pathLetterSuffix, // Xen
			"/dev/vd" + pathLetterSuffix,  // KVM
			"/dev/sd" + pathLetterSuffix,
			"/dev/nvme" + pathNumberSuffix + "n1", // Nitro instances with Noble
		}

		for _, path := range possiblePaths {
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
