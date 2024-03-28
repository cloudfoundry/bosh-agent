package fakes

type FakeDNSResolver struct {
}

func (f *FakeDNSResolver) Validate(dnsServers []string) error {
	return nil
}

func (f *FakeDNSResolver) SetupDNS(dnsServers []string) error {
	return nil
}
