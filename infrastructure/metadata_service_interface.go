package infrastructure

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type MetadataService interface {
	IsAvailable() bool
	GetPublicKey() (string, error)
	GetInstanceID() (string, error)
	GetServerName() (string, error)
	GetNetworks() (boshsettings.Networks, error)
	GetSettings() (boshsettings.Settings, error)
}

type MetadataServiceOptions struct {
	UseConfigDrive bool
}

type MetadataServiceProvider interface {
	Get() MetadataService
}

type UserDataContentsType struct {
	boshsettings.Settings

	Registry struct {
		Endpoint string
	}
	Server struct {
		Name string // Name given by CPI e.g. vm-384sd4-r7re9e...
	}
	DNS struct {
		Nameserver []string
	}
}

type DynamicMetadataService interface {
	MetadataService
	GetValueAtPath(string) (string, error)
}
