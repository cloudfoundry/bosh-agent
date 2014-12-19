package devicepathresolver

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type idDevicePathResolver struct {
	diskWaitTimeout time.Duration
	cmdRunner       boshsys.CmdRunner
	fs              boshsys.FileSystem
}

func NewIDDevicePathResolver(
	diskWaitTimeout time.Duration,
	cmdRunner boshsys.CmdRunner,
	fs boshsys.FileSystem,
) idDevicePathResolver {
	return idDevicePathResolver{
		diskWaitTimeout: diskWaitTimeout,
		cmdRunner:       cmdRunner,
		fs:              fs,
	}
}

func (idpr idDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	if diskSettings.ID == "" {
		return "", false, bosherr.Errorf("Disk ID is not set")
	}

	_, _, _, err := idpr.cmdRunner.RunCommand("udevadm", "trigger")
	if err != nil {
		return "", false, bosherr.WrapError(err, "Running udevadm trigger")
	}

	stopAfter := time.Now().Add(idpr.diskWaitTimeout)
	found := false

	var realPath string

	diskID := diskSettings.ID[0:20]

	for !found {
		if time.Now().After(stopAfter) {
			return "", true, bosherr.Errorf("Timed out getting real device path for '%s'", diskID)
		}

		time.Sleep(100 * time.Millisecond)

		deviceIDPath := filepath.Join(string(os.PathSeparator), "dev", "disk", "by-id", fmt.Sprintf("virtio-%s", diskID))

		realPath, err = idpr.fs.ReadLink(deviceIDPath)
		if err != nil {
			continue
		}

		if idpr.fs.FileExists(realPath) {
			found = true
		}
	}

	return realPath, false, nil
}
