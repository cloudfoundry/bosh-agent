package infrastructure

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type MetadataService interface {
	IsAvailable() bool
	GetPublicKey() (string, error)
	GetInstanceID() (string, error)
	GetServerName() (string, error)
	GetRegistryEndpoint() (string, error)
	GetNetworks() (boshsettings.Networks, error)
}

type MetadataServiceOptions struct {
	UseConfigDrive bool
}

type MetadataServiceProvider interface {
	Get() MetadataService
}

type UserDataContentsType struct {
	Registry struct {
		Endpoint string `json:"endpoint,omitempty"`
	} `json:"registry"`
	Server struct {
		Name string `json:"name,omitempty"` // Name given by CPI e.g. vm-384sd4-r7re9e...
	} `json:"server"`
	DNS struct {
		Nameserver []string `json:"nameserver,omitempty"`
	} `json:"dns"`
	Networks boshsettings.Networks `json:"networks,omitempty"`
}

type DynamicMetadataService interface {
	MetadataService
	GetValueAtPath(string) (string, error)
}
