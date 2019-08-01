// +build !windows

package net

import (
	"regexp"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

// cmdRoutesSearcher uses `route -n` command to list routes
// which routes in a same format on Ubuntu and CentOS
type cmdRoutesSearcher struct {
	runner boshsys.CmdRunner
}

func NewRoutesSearcher(runner boshsys.CmdRunner, _ InterfaceManager) RoutesSearcher {
	return cmdRoutesSearcher{runner}
}

func parseRoute(ipString string) Route {
	var r = regexp.MustCompile(`(?P<destination>[a-z0-9.]+)(/[0-9]+)?( via (?P<gateway>[0-9.]+))? dev (?P<interfaceName>[a-z0-9]+)`)

	match := r.FindStringSubmatch(ipString)
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
	}
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
		routes = append(routes, parseRoute(routeEntry))
	}

	return routes, nil
}
