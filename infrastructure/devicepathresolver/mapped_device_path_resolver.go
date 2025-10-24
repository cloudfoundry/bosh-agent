package devicepathresolver

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type mappedDevicePathResolver struct {
	diskWaitTimeout time.Duration
	fs              boshsys.FileSystem
	cmdRunner       boshsys.CmdRunner
}

func NewMappedDevicePathResolver(
	diskWaitTimeout time.Duration,
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner,
) DevicePathResolver {
	return mappedDevicePathResolver{
		fs:              fs,
		diskWaitTimeout: diskWaitTimeout,
		cmdRunner:       cmdRunner,
	}
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

func (dpr mappedDevicePathResolver) getRootDevicePath() (string, error) {
	mountInfo, err := dpr.fs.ReadFileString("/proc/mounts")
	if err != nil {
		return "", bosherr.WrapError(err, "Reading /proc/mounts")
	}

	mountEntries := strings.Split(mountInfo, "\n")
	for _, mountEntry := range mountEntries {
		if mountEntry == "" {
			continue
		}

		mountFields := strings.Fields(mountEntry)
		if len(mountFields) >= 2 && mountFields[1] == "/" && strings.HasPrefix(mountFields[0], "/dev/") {
			stdout, _, _, err := dpr.cmdRunner.RunCommand("readlink", "-f", mountFields[0])
			if err != nil {
				return "", bosherr.WrapError(err, "Resolving root partition path")
			}
			rootPartition := strings.TrimSpace(stdout)

			validNVMeRootPartition := regexp.MustCompile(`^/dev/[a-z]+\dn\dp\d$`)
			validSCSIRootPartition := regexp.MustCompile(`^/dev/[a-z]+\d$`)

			if validNVMeRootPartition.MatchString(rootPartition) {
				return rootPartition[:len(rootPartition)-2], nil
			} else if validSCSIRootPartition.MatchString(rootPartition) {
				return rootPartition[:len(rootPartition)-1], nil
			}
		}
	}
	return "", bosherr.Error("Root device not found")
}

func (dpr mappedDevicePathResolver) findPossibleDevice(devicePath string) (string, bool, error) {
	needsMapping := strings.HasPrefix(devicePath, "/dev/sd")

	if needsMapping { //nolint:nestif
		pathLetterSuffix := strings.Split(devicePath, "/dev/sd")[1]

		possiblePaths := []string{
			"/dev/xvd" + pathLetterSuffix, // Xen
			"/dev/vd" + pathLetterSuffix,  // KVM
			"/dev/sd" + pathLetterSuffix,
		}

		for _, path := range possiblePaths {
			if dpr.fs.FileExists(path) {
				return path, true, nil
			}
		}

		rootDevicePath, err := dpr.getRootDevicePath()
		if err != nil {
			rootDevicePath = ""
		}

		nvmePattern := "/dev/nvme%dn1"
		for i := 0; i < 10; i++ {
			nvmePath := fmt.Sprintf(nvmePattern, i)
			if dpr.fs.FileExists(nvmePath) && nvmePath != rootDevicePath {
				return nvmePath, true, nil
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
