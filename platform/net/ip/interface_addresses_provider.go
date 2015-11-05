package ip

import (
	"net"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type InterfaceAddressesProvider interface {
	Get() ([]InterfaceAddress, error)
}

type systemInterfaceAddrs struct{}

func NewSystemInterfaceAddressesProvider() InterfaceAddressesProvider {
	return &systemInterfaceAddrs{}
}

func (s *systemInterfaceAddrs) Get() ([]InterfaceAddress, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []InterfaceAddress{}, bosherr.WrapError(err, "Getting network interfaces")
	}

	interfaceAddrs := []InterfaceAddress{}
	for _, addr := range addrs {
		interfaceAddrs = append(interfaceAddrs, NewSimpleInterfaceAddress(addr.Network(), addr.String()))
	}

	return interfaceAddrs, nil
}
