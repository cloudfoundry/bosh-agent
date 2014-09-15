package infrastructure

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type Registry interface {
	GetSettings() (boshsettings.Settings, error)
}

type RegistryProvider interface {
	GetRegistry(useServerNameAsID bool) Registry
}
