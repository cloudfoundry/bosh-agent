package fakes

import (
	gonet "net"
)

type FakeResolver struct {
	GetPrimaryIPv4InterfaceName string
	GetPrimaryIPv4IPNet         *gonet.IPNet
	GetPrimaryIPv4Err           error

	GetPrimaryIPv6InterfaceName string
	GetPrimaryIPv6IPNet         *gonet.IPNet
	GetPrimaryIPv6Err           error
}

func (r *FakeResolver) GetPrimaryIP(interfaceName string, is_ipv6 bool) (*gonet.IPNet, error) {
	r.GetPrimaryIPv4InterfaceName = interfaceName
	r.GetPrimaryIPv6InterfaceName = interfaceName
	if is_ipv6 {
		return r.GetPrimaryIPv6IPNet, r.GetPrimaryIPv6Err
	}
	return r.GetPrimaryIPv4IPNet, r.GetPrimaryIPv4Err
}
