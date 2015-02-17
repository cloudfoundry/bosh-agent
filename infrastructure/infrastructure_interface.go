package infrastructure

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type Infrastructure interface {
	SetupNetworking(networks boshsettings.Networks) (err error)
	GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) string
}
