package fakes

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	"github.com/coreos/go-iptables/iptables"
)

type FakeDefaultNetworkResolver struct {
	GetDefaultNetworkNetwork    boshsettings.Network
	GetDefaultNetworkErr        error
	GetDefaultNetworkCalled     bool
	GetDefaultNetworkCalledWith iptables.Protocol
}

func (r *FakeDefaultNetworkResolver) GetDefaultNetwork(ipProtocol iptables.Protocol) (boshsettings.Network, error) {
	r.GetDefaultNetworkCalled = true
	r.GetDefaultNetworkCalledWith = ipProtocol
	return r.GetDefaultNetworkNetwork, r.GetDefaultNetworkErr
}
