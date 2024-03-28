package dnsresolver

type DNSResolver interface {
	Validate([]string) error
	SetupDNS([]string) error
}
