package fakes

import (
	gonet "net"

	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
)

type FakeReturn struct {
	IFaceName  string
	IpProtocol boship.IPProtocol
}

type FakeResolver struct {
	GetPrimaryIPInterfaceName string
	GetPrimaryIPNet           *gonet.IPNet
	GetPrimaryIPErr           error
	GetPrimaryIPCalledWith    FakeReturn
}

func (r *FakeResolver) GetPrimaryIP(interfaceName string, ipProtocol boship.IPProtocol) (*gonet.IPNet, error) {
	r.GetPrimaryIPInterfaceName = interfaceName
	r.GetPrimaryIPCalledWith.IFaceName = interfaceName
	r.GetPrimaryIPCalledWith.IpProtocol = ipProtocol
	return r.GetPrimaryIPNet, r.GetPrimaryIPErr
}
