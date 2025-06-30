package fakes

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type FakeDefaultNetworkResolver struct {
	GetDefaultNetworkNetwork    boshsettings.Network
	GetDefaultNetworkErr        error
	GetDefaultNetworkCalled     bool
	GetDefaultNetworkCalledWith bool
}

func (r *FakeDefaultNetworkResolver) GetDefaultNetwork(isIpv6 bool) (boshsettings.Network, error) {
	r.GetDefaultNetworkCalled = true
	r.GetDefaultNetworkCalledWith = isIpv6
	return r.GetDefaultNetworkNetwork, r.GetDefaultNetworkErr
}
