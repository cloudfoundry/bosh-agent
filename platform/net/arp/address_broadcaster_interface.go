package arp

import (
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
)

type AddressBroadcaster interface {
	BroadcastMACAddresses([]boship.InterfaceAddress)
}
