package fakes

import (
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type FakeDefaultNetworkResolver struct {
	GetDefaultNetworkNetwork    boshsettings.Network
	GetDefaultNetworkErr        error
	GetDefaultNetworkCalled     bool
	GetDefaultNetworkCalledWith boship.IPProtocol
}

func (r *FakeDefaultNetworkResolver) GetDefaultNetwork(ipProtocol boship.IPProtocol) (boshsettings.Network, error) {
	r.GetDefaultNetworkCalled = true
	r.GetDefaultNetworkCalledWith = ipProtocol
	return r.GetDefaultNetworkNetwork, r.GetDefaultNetworkErr
}
