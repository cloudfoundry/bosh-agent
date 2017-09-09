package ip

import (
	"fmt"
	"net"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type InterfaceAddress interface {
	GetInterfaceName() string
	// GetIP gets the exposed internet protocol address of the above interface
	GetIP() (string, error)
}

type simpleInterfaceAddress struct {
	interfaceName string
	ip            string
}

func NewSimpleInterfaceAddress(interfaceName string, ip string) InterfaceAddress {
	return simpleInterfaceAddress{interfaceName: interfaceName, ip: ip}
}

func (s simpleInterfaceAddress) GetInterfaceName() string { return s.interfaceName }

func (s simpleInterfaceAddress) GetIP() (string, error) {
	ip2 := net.ParseIP(s.ip)
	if ip2 == nil {
		return "", fmt.Errorf("Cannot parse IP '%s'", s.ip)
	}

	return fmtIP(ip2), nil
}

type resolvingInterfaceAddress struct {
	interfaceName string
	ipResolver    Resolver
	ip            string
}

func NewResolvingInterfaceAddress(
	interfaceName string,
	ipResolver Resolver,
) InterfaceAddress {
	return &resolvingInterfaceAddress{
		interfaceName: interfaceName,
		ipResolver:    ipResolver,
	}
}

func (s resolvingInterfaceAddress) GetInterfaceName() string { return s.interfaceName }

func (s *resolvingInterfaceAddress) GetIP() (string, error) {
	if s.ip != "" {
		return s.ip, nil
	}

	ip, err := s.ipResolver.GetPrimaryIPv4(s.interfaceName)
	if err != nil {
		return "", bosherr.WrapError(err, "Getting primary IPv4")
	}

	s.ip = fmtIP(ip.IP)

	return s.ip, nil
}

func fmtIP(ip net.IP) string {
	if p4 := ip.To4(); len(p4) == net.IPv4len {
		return ip.String()
	}

	return fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x",
		[]byte(ip[0:2]), []byte(ip[2:4]), []byte(ip[4:6]), []byte(ip[6:8]),
		[]byte(ip[8:10]), []byte(ip[10:12]), []byte(ip[12:14]), []byte(ip[14:16]))
}
