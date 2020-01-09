package ip

import (
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type InterfaceAddressesValidator struct {
	interfaceAddrsProvider    InterfaceAddressesProvider
	desiredInterfaceAddresses []InterfaceAddress
}

func NewInterfaceAddressesValidator(interfaceAddrsProvider InterfaceAddressesProvider, desiredInterfaceAddresses []InterfaceAddress) InterfaceAddressesValidator {
	return InterfaceAddressesValidator{
		interfaceAddrsProvider:    interfaceAddrsProvider,
		desiredInterfaceAddresses: desiredInterfaceAddresses,
	}
}

func (i InterfaceAddressesValidator) Attempt() (bool, error) {
	systemInterfaceAddresses, err := i.interfaceAddrsProvider.Get()
	if err != nil {
		return true, bosherr.WrapError(err, "Getting network interface addresses")
	}

	for _, desiredInterfaceAddress := range i.desiredInterfaceAddresses {
		ifaceName := desiredInterfaceAddress.GetInterfaceName()

		ifaces := i.findInterfaceByName(ifaceName, systemInterfaceAddresses)
		if len(ifaces) == 0 {
			return true, bosherr.Errorf("Validating network interface '%s' IP addresses, no interface configured with that name", ifaceName)
		}

		actualIPs := []string{}
		desiredIP, _ := desiredInterfaceAddress.GetIP()
		for _, iface := range ifaces {
			actualIP, _ := iface.GetIP()

			if desiredIP == actualIP {
				return false, nil
			}
			actualIPs = append(actualIPs, actualIP)
		}

		return true, bosherr.Errorf("Validating network interface '%s' IP addresses, expected: '%s', actual: [%s]", ifaceName, desiredIP, strings.Join(actualIPs, ", "))
	}

	return false, nil
}

func (i InterfaceAddressesValidator) findInterfaceByName(ifaceName string, ifaces []InterfaceAddress) []InterfaceAddress {
	result := []InterfaceAddress{}
	for _, iface := range ifaces {
		if iface.GetInterfaceName() == ifaceName {
			result = append(result, iface)
		}
	}

	return result
}
