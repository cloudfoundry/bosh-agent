package dnsresolver

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type DNSResolver interface {
	Validate([]string) error
	SetupDNS(dnsServers []string) error
	SetupSystemdResolved(boshsettings.SystemdResolvedSettings)
}
