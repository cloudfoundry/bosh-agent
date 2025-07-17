//go:build !windows
// +build !windows

package net

import (
	"regexp"
	"strings"

	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
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
	var r = regexp.MustCompile(`(?P<destination>[a-z0-9.:]+)(/[0-9]+)?( via (?P<gateway>[a-f0-9.:]+))? dev (?P<interfaceName>[a-z0-9]+)`)

	match := r.FindStringSubmatch(ipString)
	if len(match) == 0 {
		return Route{}, bosherr.Error("unexpected route")
	}
	matches := make(map[string]string)
	for i, name := range r.SubexpNames() {
		matches[name] = match[i]
	}

	destination := matches["destination"]
	gateway := matches["gateway"]

	defaultGateway := DefaultAddress

	if strings.Contains(destination, ":") || strings.Contains(gateway, ":") {
		defaultGateway = DefaultAddressIpv6
	}

	if len(gateway) == 0 {
		gateway = defaultGateway
	}

	if destination == "default" {
		destination = defaultGateway
	}

	return Route{
		Destination:   destination,
		Gateway:       gateway,
		InterfaceName: matches["interfaceName"],
	}, nil
}

func (s cmdRoutesSearcher) SearchRoutes(ipProtocol boship.IPProtocol) ([]Route, error) {
	var stdout string
	var err error
	switch ipProtocol {
	case boship.IPv4:
		stdout, _, _, err = s.runner.RunCommandQuietly("ip", "r")
		if err != nil {
			return []Route{}, bosherr.WrapError(err, "Running IPv4 route")
		}
	case boship.IPv6:
		stdout, _, _, err = s.runner.RunCommandQuietly("ip", "-6", "r")
		if err != nil {
			return []Route{}, bosherr.WrapError(err, "Running IPv6 route")
		}
	}

	routeEntries := strings.Split(stdout, "\n")
	routes := make([]Route, 0, len(routeEntries))
	for _, routeEntry := range routeEntries {
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
