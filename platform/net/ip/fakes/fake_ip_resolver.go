package fakes

import (
	gonet "net"
)

type FakeResolver struct {
	GetPrimaryIPInterfaceName string
	GetPrimaryIPIPNet         *gonet.IPNet
	GetPrimaryIPErr           error
}

func (r *FakeResolver) GetPrimaryIP(interfaceName string, is_ipv6 bool) (*gonet.IPNet, error) {
	r.GetPrimaryIPInterfaceName = interfaceName
	return r.GetPrimaryIPIPNet, r.GetPrimaryIPErr
}
