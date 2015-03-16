package net

import (
	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
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
	logger boshlog.Logger
	logTag string
}

func NewInterfaceConfigurationCreator(logger boshlog.Logger) InterfaceConfigurationCreator {
	return interfaceConfigurationCreator{
		logger: logger,
		logTag: "interfaceConfigurationCreator",
	}
}

func (creator interfaceConfigurationCreator) createInterfaceConfiguration(staticInterfaceConfigurations []StaticInterfaceConfiguration, dhcpInterfaceConfigurations []DHCPInterfaceConfiguration, ifaceName string, networkMACAddress string, networkSettings boshsettings.Network) ([]StaticInterfaceConfiguration, []DHCPInterfaceConfiguration, error) {
	creator.logger.Debug(creator.logTag, "Creating network configuration with IP: '%s', netmask: '%s'", networkSettings.IP, networkSettings.Netmask)

	if networkSettings.IP != "" && networkSettings.Netmask != "" {
		creator.logger.Debug(creator.logTag, "Using static networking")
		networkAddress, broadcastAddress, err := boshsys.CalculateNetworkAndBroadcast(networkSettings.IP, networkSettings.Netmask)
		if err != nil {
			return nil, nil, bosherr.WrapError(err, "Calculating Network and Broadcast")
		}
		staticInterfaceConfigurations = append(staticInterfaceConfigurations, StaticInterfaceConfiguration{
			Name:      ifaceName,
			Address:   networkSettings.IP,
			Netmask:   networkSettings.Netmask,
			Network:   networkAddress,
			Broadcast: broadcastAddress,
			Mac:       networkMACAddress,
			Gateway:   networkSettings.Gateway,
		})
	} else {
		creator.logger.Debug(creator.logTag, "Using dhcp networking")
		dhcpInterfaceConfigurations = append(dhcpInterfaceConfigurations, DHCPInterfaceConfiguration{
			Name: ifaceName,
		})
	}
	return staticInterfaceConfigurations, dhcpInterfaceConfigurations, nil
}

func (creator interfaceConfigurationCreator) CreateInterfaceConfigurations(networks boshsettings.Networks, interfacesByMAC map[string]string) ([]StaticInterfaceConfiguration, []DHCPInterfaceConfiguration, error) {
	var (
		staticInterfaceConfigurations []StaticInterfaceConfiguration
		dhcpInterfaceConfigurations   []DHCPInterfaceConfiguration
		err                           error
	)

	// In cases where we only have one network and it has no MAC address (either because the IAAS doesn't give us one or
	// it's an old CPI), if we only have one interface, we should map them
	if len(networks) == 1 {
		_, networkSettings := creator.getTheOnlyNetwork(networks)
		if networkSettings.Mac == "" && len(interfacesByMAC) == 1 {
			networkMACAddress, ifaceName := creator.getTheOnlyInterface(interfacesByMAC)
			staticInterfaceConfigurations, dhcpInterfaceConfigurations, err = creator.createInterfaceConfiguration(staticInterfaceConfigurations, dhcpInterfaceConfigurations, ifaceName, networkMACAddress, networkSettings)
			if err != nil {
				return nil, nil, bosherr.WrapError(err, "Creating interface configuration")
			}
			return staticInterfaceConfigurations, dhcpInterfaceConfigurations, nil
		}
	}

	// otherwise map all the networks by MAC address
	for networkName, networkSettings := range networks {
		if networkSettings.Mac == "" {
			return nil, nil, bosherr.Errorf("Network '%s' doesn't specify a MAC address", networkName)
		}

		ifaceName, found := interfacesByMAC[networkSettings.Mac]
		if !found {
			return nil, nil, bosherr.Errorf("No interface exists with MAC address '%s'", networkSettings.Mac)
		}

		staticInterfaceConfigurations, dhcpInterfaceConfigurations, err = creator.createInterfaceConfiguration(staticInterfaceConfigurations, dhcpInterfaceConfigurations, ifaceName, networkSettings.Mac, networkSettings)
		if err != nil {
			return nil, nil, bosherr.WrapError(err, "Creating interface configuration")
		}
	}
	return staticInterfaceConfigurations, dhcpInterfaceConfigurations, nil
}

func (creator interfaceConfigurationCreator) getTheOnlyNetwork(networks boshsettings.Networks) (string, boshsettings.Network) {
	for networkName, networkSettings := range networks {
		return networkName, networkSettings
	}
	return "", boshsettings.Network{}
}

func (creator interfaceConfigurationCreator) getTheOnlyInterface(interfacesByMAC map[string]string) (string, string) {
	for mac, iface := range interfacesByMAC {
		return mac, iface
	}
	return "", ""
}
