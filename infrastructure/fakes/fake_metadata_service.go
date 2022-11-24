package fakes

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type FakeMetadataService struct {
	LoadErr error

	PublicKey       string
	GetPublicKeyErr error

	InstanceID       string
	GetInstanceIDErr error

	ServerName       string
	GetServerNameErr error

	Networks    boshsettings.Networks
	NetworksErr error

	Settings    boshsettings.Settings
	SettingsErr error

	Available bool
}

func (ms FakeMetadataService) Load() error {
	return ms.LoadErr
}

func (ms FakeMetadataService) GetPublicKey() (string, error) {
	return ms.PublicKey, ms.GetPublicKeyErr
}

func (ms FakeMetadataService) GetInstanceID() (string, error) {
	return ms.InstanceID, ms.GetInstanceIDErr
}

func (ms FakeMetadataService) GetServerName() (string, error) {
	return ms.ServerName, ms.GetServerNameErr
}

func (ms FakeMetadataService) GetNetworks() (boshsettings.Networks, error) {
	return ms.Networks, ms.NetworksErr
}

func (ms FakeMetadataService) GetSettings() (boshsettings.Settings, error) {
	return ms.Settings, ms.SettingsErr
}

func (ms FakeMetadataService) IsAvailable() bool {
	return ms.Available
}
