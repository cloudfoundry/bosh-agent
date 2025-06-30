package net

type Route struct {
	Destination   string
	Gateway       string
	Netmask       string
	InterfaceName string
}

type RoutesSearcher interface {
	SearchRoutes(ipv6 bool) ([]Route, error)
}

const DefaultAddress = `0.0.0.0`
const DefaultAddressIpv6 = `::`

func (r Route) IsDefault(isIpv6 bool) bool {
	if isIpv6 {
		return r.Destination == DefaultAddressIpv6
	}
	return r.Destination == DefaultAddress
}
