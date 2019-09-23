package fakes

import (
	"sync"

	boship "github.com/cloudfoundry/bosh-agent/platform/net/ip"
)

type FakeAddressBroadcaster struct {
	mux                            sync.Mutex
	broadcastMACAddressesAddresses []boship.InterfaceAddress
}

func (b *FakeAddressBroadcaster) BroadcastMACAddresses(addresses []boship.InterfaceAddress) {
	b.mux.Lock()
	b.broadcastMACAddressesAddresses = addresses
	b.mux.Unlock()
}

func (b *FakeAddressBroadcaster) Value() []boship.InterfaceAddress {
	b.mux.Lock()
	defer b.mux.Unlock()

	return b.broadcastMACAddressesAddresses
}
