package ip

import (
	gonet "net"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type InterfaceToAddrsFunc func(string) ([]gonet.Addr, error)

func NetworkInterfaceToAddrsFunc(interfaceName string) ([]gonet.Addr, error) {
	iface, err := gonet.InterfaceByName(interfaceName)
	if err != nil {
		return []gonet.Addr{}, bosherr.WrapErrorf(err, "Searching for '%s' interface", interfaceName)
	}

	return iface.Addrs()
}

type Resolver interface {
	// GetPrimaryIP always returns error unless IPNet is found for given interface
	GetPrimaryIP(interfaceName string, ipProtocol IPProtocol) (*gonet.IPNet, error)
}

type ipResolver struct {
	ifaceToAddrsFunc InterfaceToAddrsFunc
}

func NewResolver(ifaceToAddrsFunc InterfaceToAddrsFunc) Resolver {
	return ipResolver{ifaceToAddrsFunc: ifaceToAddrsFunc}
}

func (r ipResolver) GetPrimaryIP(interfaceName string, ipProtocol IPProtocol) (*gonet.IPNet, error) {
	addrs, err := r.ifaceToAddrsFunc(interfaceName)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Looking up addresses for interface '%s'", interfaceName)
	}

	if len(addrs) == 0 {
		return nil, bosherr.Errorf("No addresses found for interface '%s'", interfaceName)
	}

	var foundNonGlobalUniCast *gonet.IPNet

	for _, addr := range addrs {
		ip, ok := addr.(*gonet.IPNet)

		if !ok {
			continue
		}

		if ip.IP.To16() != nil && ip.IP.IsGlobalUnicast() && ipProtocol == IPv6 {
			return ip, nil
		}

		if ip.IP.To4() != nil && ipProtocol == IPv4 {
			if ip.IP.IsGlobalUnicast() {
				return ip, nil
			} else {
				if foundNonGlobalUniCast == nil {
					foundNonGlobalUniCast = ip
				}
			}
		}
	}

	ipVersion := 4
	if ipProtocol == IPv6 {
		ipVersion = 6
	}

	if foundNonGlobalUniCast != nil {
		return foundNonGlobalUniCast, nil
	}

	return nil, bosherr.Errorf("Failed to find primary address IPv%d for interface '%s'", ipVersion, interfaceName)
}
