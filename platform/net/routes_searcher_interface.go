package net

import (
	"github.com/coreos/go-iptables/iptables"
)

type Route struct {
	Destination   string
	Gateway       string
	Netmask       string
	InterfaceName string
}

type RoutesSearcher interface {
	SearchRoutes(ipProtocol iptables.Protocol) ([]Route, error)
}

const DefaultAddress = `0.0.0.0`
const DefaultAddressIpv6 = `::`

func (r Route) IsDefault(ipProtocol iptables.Protocol) bool {
	var isDefault bool

	switch ipProtocol {
	case iptables.ProtocolIPv4:
		isDefault = r.Destination == DefaultAddress
	case iptables.ProtocolIPv6:
		isDefault = r.Destination == DefaultAddressIpv6
	}
	return isDefault
}
