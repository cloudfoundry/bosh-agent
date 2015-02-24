package net

import (
	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type StaticInterfaceConfiguration struct {
	Name      string
	Address   string
	Netmask   string
	Network   string
	Broadcast string
	Mac       string
	Gateway   string
}

type DHCPInterfaceConfiguration struct {
	Name string
}

type InterfaceConfigurationCreator interface {
	CreateInterfaceConfigurations(boshsettings.Networks, map[string]string) ([]StaticInterfaceConfiguration, []DHCPInterfaceConfiguration, error)
}

type interfaceConfigurationCreator struct {
}

func NewInterfaceConfigurationCreator() InterfaceConfigurationCreator {
	return interfaceConfigurationCreator{}
}

func (creator interfaceConfigurationCreator) CreateInterfaceConfigurations(networks boshsettings.Networks, interfacesByMAC map[string]string) ([]StaticInterfaceConfiguration, []DHCPInterfaceConfiguration, error) {
	var (
		staticInterfaceConfigurations []StaticInterfaceConfiguration
		dhcpInterfaceConfigurations   []DHCPInterfaceConfiguration
	)

	// if one network
	//   if mac specified in settings
	//     if interface with mac exists
	//       match
	//     else
	//       blow up
	//   else
	//    match just the one
	// if many networks
	//   if mac specified in settings
	//     if interface with mac exists
	//       match
	//     else
	//       blow up
	//   else
	//     blow up

	for _, network := range networks {
		ifaceName := interfacesByMAC[network.Mac]

		if network.IsDynamic() {
			dhcpInterfaceConfigurations = append(dhcpInterfaceConfigurations, DHCPInterfaceConfiguration{
				Name: ifaceName,
			})
		} else {
			networkAddress, broadcastAddress, err := boshsys.CalculateNetworkAndBroadcast(network.IP, network.Netmask)
			if err != nil {
				return nil, nil, bosherr.WrapError(err, "Calculating Network and Broadcast")
			}
			staticInterfaceConfigurations = append(staticInterfaceConfigurations, StaticInterfaceConfiguration{
				Name:      ifaceName,
				Address:   network.IP,
				Netmask:   network.Netmask,
				Network:   networkAddress,
				Broadcast: broadcastAddress,
				Mac:       network.Mac,
				Gateway:   network.Gateway,
			})
		}
	}

	return staticInterfaceConfigurations, dhcpInterfaceConfigurations, nil
}
