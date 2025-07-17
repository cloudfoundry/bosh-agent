package net

import (
	gonet "net"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type defaultNetworkResolver struct {
	routesSearcher RoutesSearcher
	ipResolver     boship.Resolver
}

func NewDefaultNetworkResolver(
	routesSearcher RoutesSearcher,
	ipResolver boship.Resolver,
) boshsettings.DefaultNetworkResolver {
	return defaultNetworkResolver{
		routesSearcher: routesSearcher,
		ipResolver:     ipResolver,
	}
}

func (r defaultNetworkResolver) GetDefaultNetwork(ipProtocol boship.IPProtocol) (boshsettings.Network, error) {
	network := boshsettings.Network{}

	routes, err := r.routesSearcher.SearchRoutes(ipProtocol)

	if err != nil {
		return network, bosherr.WrapError(err, "Searching routes")
	}

	if len(routes) == 0 {
		return network, bosherr.Error("No routes found")
	}

	for _, route := range routes {
		if !route.IsDefault(ipProtocol) {
			continue
		}

		ip, err := r.ipResolver.GetPrimaryIP(route.InterfaceName, ipProtocol)

		if err != nil {
			var ipVersion int

			switch ipProtocol {
			case boship.IPv4:
				ipVersion = 4
			case boship.IPv6:
				ipVersion = 6
			}
			return network, bosherr.WrapErrorf(err, "Getting primary IPv%d for interface '%s'", ipVersion, route.InterfaceName)
		}

		return boshsettings.Network{
			IP:      ip.IP.String(),
			Netmask: gonet.IP(ip.Mask).String(),
			Gateway: route.Gateway,
		}, nil
	}

	return network, bosherr.Error("Failed to find default route")
}
