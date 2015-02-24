package fakes

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type FakeManager struct {
	FakeDefaultNetworkResolver

	SetupManualNetworkingNetworks boshsettings.Networks
	SetupManualNetworkingErr      error

	SetupNetworkingNetworks boshsettings.Networks
	SetupNetworkingErr      error

	SetupDhcpNetworks boshsettings.Networks
	SetupDhcpErr      error
}

func (net *FakeManager) SetupManualNetworking(networks boshsettings.Networks, errCh chan error) error {
	net.SetupManualNetworkingNetworks = networks
	return net.SetupManualNetworkingErr
}
func (net *FakeManager) SetupNetworking(networks boshsettings.Networks, errCh chan error) error {
	net.SetupNetworkingNetworks = networks
	return net.SetupNetworkingErr
}

func (net *FakeManager) SetupDhcp(networks boshsettings.Networks, errCh chan error) error {
	net.SetupDhcpNetworks = networks
	return net.SetupDhcpErr
}
