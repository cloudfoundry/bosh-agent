package fakes

import (
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
)

type FakeInterfaceAddressesProvider struct {
	GetInterfaceAddresses []boship.InterfaceAddress
	GetErr                error
}

func (f *FakeInterfaceAddressesProvider) Get() ([]boship.InterfaceAddress, error) {
	return f.GetInterfaceAddresses, f.GetErr
}
