package infrastructure

import (
	gonet "net"
	"net/url"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type registryEndpointResolver struct {
	delegate DNSResolver
}

func NewRegistryEndpointResolver(resolver DNSResolver) DNSResolver {
	return registryEndpointResolver{
		delegate: resolver,
	}
}

func (r registryEndpointResolver) LookupHost(dnsServers []string, endpoint string) (string, error) {
	registryURL, err := url.Parse(endpoint)
	if err != nil {
		return "", bosherr.WrapError(err, "Parsing registry named endpoint")
	}

	host, port, err := gonet.SplitHostPort(registryURL.Host)
	if err != nil {
		return "", bosherr.WrapError(err, "Splitting registry host")
	}

	registryIP, err := r.delegate.LookupHost(dnsServers, host)
	if err != nil {
		return "", bosherr.WrapError(err, "Looking up registry")
	}

	if len(port) > 0 {
		registryURL.Host = gonet.JoinHostPort(registryIP, port)
	} else {
		registryURL.Host = registryIP
	}

	return registryURL.String(), nil
}
