package infrastructure

import (
	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type Infrastructure interface {
	SetupSSH(username string) (err error)
	GetSettings() (settings boshsettings.Settings, err error)
	SetupNetworking(networks boshsettings.Networks) (err error)
	GetEphemeralDiskPath(devicePath string) (realPath string, found bool)
	GetDevicePathResolver() (devicePathResolver boshdpresolv.DevicePathResolver)
}
