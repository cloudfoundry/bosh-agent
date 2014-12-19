package infrastructure

import (
	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type Infrastructure interface {
	SetupSSH(username string) (err error)
	GetSettings() (settings boshsettings.Settings, err error)
	SetupNetworking(networks boshsettings.Networks) (err error)
	GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) string
	GetDevicePathResolver() (devicePathResolver boshdpresolv.DevicePathResolver)
}
