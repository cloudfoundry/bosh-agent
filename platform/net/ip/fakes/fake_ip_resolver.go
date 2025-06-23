package fakes

import (
	"fmt"
	gonet "net"
)

type FakeResolver struct {
	GetPrimaryIPInterfaceName string
	GetPrimaryIPNet           *gonet.IPNet
	GetPrimaryIPErr           error
	GetPrimaryIPCalledWith    []string
}

func (r *FakeResolver) GetPrimaryIP(interfaceName string, is_ipv6 bool) (*gonet.IPNet, error) {
	r.GetPrimaryIPInterfaceName = interfaceName
	r.GetPrimaryIPCalledWith = append(r.GetPrimaryIPCalledWith, interfaceName)
	r.GetPrimaryIPCalledWith = append(r.GetPrimaryIPCalledWith, fmt.Sprintf("%t", is_ipv6))
	return r.GetPrimaryIPNet, r.GetPrimaryIPErr
}
