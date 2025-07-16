package fakes

import (
	gonet "net"

	"github.com/coreos/go-iptables/iptables"
)

type FakeReturn struct {
	IFaceName  string
	IpProtocol iptables.Protocol
}

type FakeResolver struct {
	GetPrimaryIPInterfaceName string
	GetPrimaryIPNet           *gonet.IPNet
	GetPrimaryIPErr           error
	GetPrimaryIPCalledWith    FakeReturn
}

func (r *FakeResolver) GetPrimaryIP(interfaceName string, ipProtocol iptables.Protocol) (*gonet.IPNet, error) {
	r.GetPrimaryIPInterfaceName = interfaceName
	r.GetPrimaryIPCalledWith.IFaceName = interfaceName
	r.GetPrimaryIPCalledWith.IpProtocol = ipProtocol
	return r.GetPrimaryIPNet, r.GetPrimaryIPErr
}
