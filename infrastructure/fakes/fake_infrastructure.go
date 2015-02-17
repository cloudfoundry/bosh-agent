package fakes

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type FakeInfrastructure struct {
	Settings                boshsettings.Settings
	SetupSSHUsername        string
	SetupNetworkingNetworks boshsettings.Networks

	GetEphemeralDiskSettings     boshsettings.DiskSettings
	GetEphemeralDiskPathFound    bool
	GetEphemeralDiskPathRealPath string
}

func NewFakeInfrastructure() (infrastructure *FakeInfrastructure) {
	infrastructure = &FakeInfrastructure{}
	infrastructure.Settings = boshsettings.Settings{}
	return
}

func (i *FakeInfrastructure) SetupSSH(username string) (err error) {
	i.SetupSSHUsername = username
	return
}

func (i *FakeInfrastructure) GetSettings() (settings boshsettings.Settings, err error) {
	settings = i.Settings
	return
}

func (i *FakeInfrastructure) SetupNetworking(networks boshsettings.Networks) (err error) {
	i.SetupNetworkingNetworks = networks
	return
}

func (i *FakeInfrastructure) GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) string {
	i.GetEphemeralDiskSettings = diskSettings
	return i.GetEphemeralDiskPathRealPath
}
