package net

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type DefaultNetworkResolver interface {
	// Ideally we would find a network based on a MAC address
	// but current CPI implementations do not include it
	GetDefaultNetwork() (boshsettings.Network, error)
}

type Manager interface {
	// SetupNetworking configures network interfaces with either a static ip or dhcp.
	// If errCh is provided, nil or an error will be sent
	// upon completion of background network reconfiguration (e.g. arping).
	SetupNetworking(networks boshsettings.Networks, errCh chan error) error

	DefaultNetworkResolver
}
