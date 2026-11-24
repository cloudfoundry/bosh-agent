package fakes

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type FakeDNSResolver struct {
	SetupSystemdResolvedConfig boshsettings.SystemdResolvedSettings
}

func (f *FakeDNSResolver) Validate(dnsServers []string) error {
	return nil
}

func (f *FakeDNSResolver) SetupDNS(dnsServers []string) error {
	return nil
}

func (f *FakeDNSResolver) SetupSystemdResolved(resolved boshsettings.SystemdResolvedSettings) {
	f.SetupSystemdResolvedConfig = resolved
}
