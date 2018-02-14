package net

import (
	"syscall"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

var (
	modiphlpapi         = syscall.MustLoadDLL("iphlpapi.dll")
	procGetAdaptersInfo = modiphlpapi.MustFindProc("GetAdaptersInfo")
)

type windowsRoutesSearcher struct {
	interfaceManager InterfaceManager
	cmdRunner        boshsys.CmdRunner
}

func NewRoutesSearcher(cmdRunner boshsys.CmdRunner, interfaceManager InterfaceManager) RoutesSearcher {
	return windowsRoutesSearcher{interfaceManager, cmdRunner}
}

func (s windowsRoutesSearcher) SearchRoutes() ([]Route, error) {
	ifs, err := s.interfaceManager.GetInterfaces()
	if err != nil {
		return nil, bosherr.WrapError(err, "Running route")
	}
	var routes []Route
	defaultGateway, _, _, err := s.cmdRunner.RunCommandQuietly("(Get-NetRoute -DestinationPrefix '0.0.0.0/0').NextHop")
	if err != nil {
		return nil, bosherr.WrapError(err, "Running route")
	}
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
