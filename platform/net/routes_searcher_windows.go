package net

import (
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type windowsRoutesSearcher struct {
	interfaceManager InterfaceManager
	cmdRunner        boshsys.CmdRunner
}

func NewRoutesSearcher(_ boshlog.Logger, cmdRunner boshsys.CmdRunner, interfaceManager InterfaceManager) RoutesSearcher {
	return windowsRoutesSearcher{interfaceManager, cmdRunner}
}

func (s windowsRoutesSearcher) SearchRoutes(ipProtocol boship.IPProtocol) ([]Route, error) {
	var err error

	ifs, err := s.interfaceManager.GetInterfaces()
	if err != nil {
		return nil, bosherr.WrapError(err, "Running route")
	}

	var defaultGateway string

	switch ipProtocol {
	case boship.IPv4:
		defaultGateway, _, _, err = s.cmdRunner.RunCommandQuietly("(Get-NetRoute -DestinationPrefix '0.0.0.0/0').NextHop")
		if err != nil {
			return nil, bosherr.WrapError(err, "Running IPv4 route")
		}
	case boship.IPv6:
		defaultGateway, _, _, err = s.cmdRunner.RunCommandQuietly("(Get-NetRoute -DestinationPrefix '::/0').NextHop")
		if err != nil {
			return nil, bosherr.WrapError(err, "Running IPv6 route")
		}
	}

	routes := make([]Route, 0, len(ifs))
	for _, fs := range ifs {
		route := Route{
			InterfaceName: fs.Name,
			Gateway:       fs.Gateway,
		}
		if fs.Gateway == defaultGateway {
			if ipProtocol == boship.IPv6 {
				route.Destination = "::" // Default route for IPv6
			} else {
				route.Destination = "0.0.0.0" // Default route for IPv4
			}
		}
		routes = append(routes, route)
	}

	if len(routes) == 0 {
		return nil, bosherr.Error("no routes")
	}

	return routes, nil
}
