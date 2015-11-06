package net

import (
	"fmt"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type DNSValidator interface {
	Validate([]string) error
}

type dnsValidator struct {
	fs boshsys.FileSystem
}

func NewDNSValidator(fs boshsys.FileSystem) DNSValidator {
	return &dnsValidator{
		fs: fs,
	}
}

func (d *dnsValidator) Validate(dnsServers []string) error {
	resolvConfContents, err := d.fs.ReadFileString("/etc/resolv.conf")
	if err != nil {
		return bosherr.WrapError(err, "Reading /etc/resolv.conf")
	}

	for _, dnsServer := range dnsServers {
		if !strings.Contains(resolvConfContents, dnsServer) {
			return bosherr.WrapError(err, fmt.Sprintf("Nameserver '%s' is not included in /etc/resolv.conf", dnsServer))
		}
	}

	return nil
}
