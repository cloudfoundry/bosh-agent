package net

import (
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

func (s windowsRoutesSearcher) SearchRoutes() ([]Route, error) {
	ifs, err := s.interfaceManager.GetInterfaces()
	if err != nil {
		return nil, bosherr.WrapError(err, "Running route")
	}

	defaultGateway, _, _, err := s.cmdRunner.RunCommandQuietly("(Get-NetRoute -DestinationPrefix '0.0.0.0/0').NextHop")
	if err != nil {
		return nil, bosherr.WrapError(err, "Running route")
	}

	routes := make([]Route, 0, len(ifs))
	for _, fs := range ifs {
		route := Route{
			InterfaceName: fs.Name,
			Gateway:       fs.Gateway,
		}
		if fs.Gateway == defaultGateway {
			route.Destination = "0.0.0.0"
		}
		routes = append(routes, route)
	}

	if len(routes) == 0 {
		return nil, bosherr.Error("no routes")
	}

	return routes, nil
}
