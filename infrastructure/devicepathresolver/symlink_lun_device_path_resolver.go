package devicepathresolver

import (
	"path"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type SymlinkLunDevicePathResolver struct {
	diskWaitTimeout time.Duration
	basePath        string
	fs              boshsys.FileSystem

	logTag string
	logger boshlog.Logger
}

func NewSymlinkLunDevicePathResolver(
	basePath string,
	diskWaitTimeout time.Duration,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) SymlinkLunDevicePathResolver {
	return SymlinkLunDevicePathResolver{
		basePath:        basePath,
		diskWaitTimeout: diskWaitTimeout,
		fs:              fs,

		logTag: "symlinkLunResolver",
		logger: logger,
	}
}

func (r SymlinkLunDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	if diskSettings.Lun == "" {
		return "", false, bosherr.Error("Disk lun is not set")
	}

	lunSymlink := path.Join(r.basePath, diskSettings.Lun)
	r.logger.Debug(r.logTag, "Looking up LUN symlink '%s'", lunSymlink)

	stopAfter := time.Now().Add(r.diskWaitTimeout)

	for {
		if time.Now().After(stopAfter) {
			return "", true, bosherr.Errorf("Timed out waiting for symlink '%s' to resolve", lunSymlink)
		}

		realPath, err := r.fs.ReadAndFollowLink(lunSymlink)
		if err != nil {
			r.logger.Debug(r.logTag, "Symlink '%s' not yet available: %s", lunSymlink, err.Error())
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if r.fs.FileExists(realPath) {
			r.logger.Debug(r.logTag, "Resolved LUN symlink '%s' to real path '%s'", lunSymlink, realPath)
			return realPath, false, nil
		}

		r.logger.Debug(r.logTag, "Real path '%s' does not yet exist", realPath)
		time.Sleep(100 * time.Millisecond)
	}
}
