// +build !windows

package net

import (
	"regexp"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

// cmdRoutesSearcher uses `route -n` command to list routes
// which routes in a same format on Ubuntu and CentOS
type cmdRoutesSearcher struct {
	runner boshsys.CmdRunner
	logger boshlog.Logger
}

func NewRoutesSearcher(logger boshlog.Logger, runner boshsys.CmdRunner, _ InterfaceManager) RoutesSearcher {
	return cmdRoutesSearcher{
		runner: runner,
		logger: logger,
	}
}

func parseRoute(ipString string) (Route, error) {
	var r = regexp.MustCompile(`(?P<destination>[a-z0-9.]+)(/[0-9]+)?( via (?P<gateway>[0-9.]+))? dev (?P<interfaceName>[a-z0-9]+)`)

	match := r.FindStringSubmatch(ipString)
	if len(match) == 0 {
		return Route{}, bosherr.Error("unexpected route")
	}
	matches := make(map[string]string)
	for i, name := range r.SubexpNames() {
		matches[name] = match[i]
	}
	gateway := DefaultAddress
	if len(matches["gateway"]) > 0 {
		gateway = matches["gateway"]
	}

	destination := matches["destination"]
	if destination == "default" {
		destination = DefaultAddress
	}

	return Route{
		Destination:   destination,
		Gateway:       gateway,
		InterfaceName: matches["interfaceName"],
	}, nil
}

func (s cmdRoutesSearcher) SearchRoutes() ([]Route, error) {
	var routes []Route

	stdout, _, _, err := s.runner.RunCommandQuietly("ip", "r")
	if err != nil {
		return routes, bosherr.WrapError(err, "Running route")
	}

	for _, routeEntry := range strings.Split(stdout, "\n") {
		if len(routeEntry) == 0 {
			continue
		}
		route, err := parseRoute(routeEntry)
		if err != nil {
			s.logger.Warn("SearchRoutes", "parseRoute error for route '%s': %s", routeEntry, err.Error())
			continue
		}
		routes = append(routes, route)
	}

	return routes, nil
}
