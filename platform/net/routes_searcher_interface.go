package net

import (
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
)

type Route struct {
	Destination   string
	Gateway       string
	Netmask       string
	InterfaceName string
}

type RoutesSearcher interface {
	SearchRoutes(ipProtocol boship.IPProtocol) ([]Route, error)
}

const DefaultAddress = `0.0.0.0`
const DefaultAddressIpv6 = `::`

func (r Route) IsDefault(ipProtocol boship.IPProtocol) bool {
	var isDefault bool

	switch ipProtocol {
	case boship.IPv4:
		isDefault = r.Destination == DefaultAddress
	case boship.IPv6:
		isDefault = r.Destination == DefaultAddressIpv6
	}
	return isDefault
}
