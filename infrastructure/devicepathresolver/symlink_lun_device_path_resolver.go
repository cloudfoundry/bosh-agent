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
	symlinkResolver *SymlinkDeviceResolver

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
		symlinkResolver: NewSymlinkDeviceResolver(fs, logger),

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

	realPath, err := r.symlinkResolver.WaitForSymlink(lunSymlink, r.diskWaitTimeout)
	if err != nil {
		return "", true, err
	}

	r.logger.Debug(r.logTag, "Resolved LUN symlink '%s' to real path '%s'", lunSymlink, realPath)
	return realPath, false, nil
}
