package ip

import (
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type InterfaceAddressesValidator interface {
	Validate(desiredInterfaceAddresses []InterfaceAddress) error
}

type interfaceAddressesValidator struct {
	interfaceAddrsProvider InterfaceAddressesProvider
}

func NewInterfaceAddressesValidator(interfaceAddrsProvider InterfaceAddressesProvider) InterfaceAddressesValidator {
	return &interfaceAddressesValidator{
		interfaceAddrsProvider: interfaceAddrsProvider,
	}
}

func (i *interfaceAddressesValidator) Validate(desiredInterfaceAddresses []InterfaceAddress) error {
	systemInterfaceAddresses, err := i.interfaceAddrsProvider.Get()
	if err != nil {
		return bosherr.WrapError(err, "Getting network interface addresses")
	}

	for _, desiredInterfaceAddress := range desiredInterfaceAddresses {
		ifaceName := desiredInterfaceAddress.GetInterfaceName()

		ifaces := i.findInterfaceByName(ifaceName, systemInterfaceAddresses)
		if len(ifaces) == 0 {
			return bosherr.Errorf("Validating network interface '%s' IP addresses, no interface configured with that name", ifaceName)
		}

		actualIPs := []string{}
		desiredIP, _ := desiredInterfaceAddress.GetIP()
		for _, iface := range ifaces {
			actualIP, _ := iface.GetIP()

			if desiredIP == actualIP {
				return nil
			}
			actualIPs = append(actualIPs, actualIP)
		}

		return bosherr.Errorf("Validating network interface '%s' IP addresses, expected: '%s', actual: [%s]", ifaceName, desiredIP, strings.Join(actualIPs, ", "))
	}

	return nil
}

func (i *interfaceAddressesValidator) findInterfaceByName(ifaceName string, ifaces []InterfaceAddress) []InterfaceAddress {
	result := []InterfaceAddress{}
	for _, iface := range ifaces {
		if iface.GetInterfaceName() == ifaceName {
			result = append(result, iface)
		}
	}

	return result
}
