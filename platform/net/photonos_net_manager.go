// Copyright © 2018 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache License 2.0

package net

import (
	"strings"
	"strconv"
	ipnet "net"
	bosharp "github.com/cloudfoundry/bosh-agent/platform/net/arp"
	boship "github.com/cloudfoundry/bosh-agent/platform/net/ip"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const photonosNetManagerLogTag = "photonosNetManager"

type photonosNetManager struct {
	fs                            boshsys.FileSystem
	cmdRunner                     boshsys.CmdRunner
	routesSearcher                RoutesSearcher
	ipResolver                    boship.Resolver
	interfaceConfigurationCreator InterfaceConfigurationCreator
	interfaceAddressesValidator   boship.InterfaceAddressesValidator
	dnsValidator                  DNSValidator
	addressBroadcaster            bosharp.AddressBroadcaster
	logger                        boshlog.Logger
}

func NewPhotonosNetManager(
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner,
	ipResolver boship.Resolver,
	interfaceConfigurationCreator InterfaceConfigurationCreator,
	interfaceAddressesValidator boship.InterfaceAddressesValidator,
	dnsValidator DNSValidator,
	addressBroadcaster bosharp.AddressBroadcaster,
	logger boshlog.Logger,
) Manager {
	return photonosNetManager{
		fs:                            fs,
		cmdRunner:                     cmdRunner,
		ipResolver:                    ipResolver,
		interfaceConfigurationCreator: interfaceConfigurationCreator,
		interfaceAddressesValidator:   interfaceAddressesValidator,
		dnsValidator:                  dnsValidator,
		addressBroadcaster:            addressBroadcaster,
		logger:                        logger,
	}
}

func (net photonosNetManager) SetupIPv6(_ boshsettings.IPv6, _ <-chan struct{}) error { return nil }

func (net photonosNetManager) SetupNetworking(networks boshsettings.Networks, errCh chan error) error {
	nonVipNetworks := boshsettings.Networks{}
	for networkName, networkSettings := range networks {
		if networkSettings.IsVIP() {
			continue
		}
		nonVipNetworks[networkName] = networkSettings
	}

	staticInterfaceConfigurations, dhcpInterfaceConfigurations, err := net.buildInterfaces(nonVipNetworks)
	if err != nil {
		return err
	}

	dnsNetwork, _ := nonVipNetworks.DefaultNetworkFor("dns")
	dnsServers := dnsNetwork.DNS

	err = net.writeNetworkInterfaces(dhcpInterfaceConfigurations, staticInterfaceConfigurations, dnsServers)
	if err != nil {
		return bosherr.WrapError(err, "Writing network configuration")
	}

	net.restartNetworkingInterfaces()

	staticAddresses, dynamicAddresses := net.ifaceAddresses(staticInterfaceConfigurations, dhcpInterfaceConfigurations)

	err = net.interfaceAddressesValidator.Validate(staticAddresses)
	if err != nil {
		return bosherr.WrapError(err, "Validating static network configuration")
	}

	err = net.dnsValidator.Validate(dnsServers)
	if err != nil {
		return bosherr.WrapError(err, "Validating dns configuration")
	}

	net.broadcastIps(append(staticAddresses, dynamicAddresses...), errCh)

	return nil
}

func (net photonosNetManager) GetConfiguredNetworkInterfaces() ([]string, error) {
	interfaces := []string{}

	interfacesByMacAddress, err := net.detectMacAddresses()
	if err != nil {
		return interfaces, bosherr.WrapError(err, "Getting network interfaces")
	}

	for _, iface := range interfacesByMacAddress {
		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

func (net photonosNetManager) maskToCIDR (netMask string) int {
        maskSize := ipnet.IPMask(ipnet.ParseIP(netMask).To4())
	cidr, _ := maskSize.Size()
	return cidr
}

func (net photonosNetManager) writeNetworkInterfaces(dhcpInterfaceConfigurations []DHCPInterfaceConfiguration, staticInterfaceConfigurations []StaticInterfaceConfiguration, dnsServers []string) error {

	for _, interfaceConfigInfo := range staticInterfaceConfigurations {

		_, _, _, err := net.cmdRunner.RunCommand("netmgr", "ip4_address", "--set", "--interface", interfaceConfigInfo.Name,
                                                              "--mode", "static", "--addr", interfaceConfigInfo.Address + "/" + strconv.Itoa(net.maskToCIDR(interfaceConfigInfo.Netmask)),
                                                              "--gateway", interfaceConfigInfo.Gateway)
		if err != nil {
			return bosherr.WrapErrorf(err, "Setting Address '%s' and Gateway '%s' failed", interfaceConfigInfo.Name, interfaceConfigInfo.Gateway)
		}

		_, _, _, err = net.cmdRunner.RunCommand("netmgr", "link_info", "--set", "--interface", interfaceConfigInfo.Name,
                                                              "--macaddr", interfaceConfigInfo.Mac)
		if err != nil {
			return bosherr.WrapError(err, "Setting Mac Address failed")
		}
	}

	for _, dhcpConfig := range dhcpInterfaceConfigurations {

		_, _, _, err := net.cmdRunner.RunCommand("netmgr", "ip4_address", "--set", "--interface", dhcpConfig.Name,
                                                              "--mode", "dhcp")
		if err != nil {
			return bosherr.WrapErrorf(err, "Setting DHCP as IP option for the interface '%s' failed", dhcpConfig.Name)
		}

	}

	for i, dnsServer := range dnsServers {
		if i == 0 {
			_, _, _, err := net.cmdRunner.RunCommand("netmgr", "dns_servers", "--set", "--mode", "static",
                                                              "--servers", dnsServer)
			if err != nil {
				return bosherr.WrapErrorf(err, "Setting DNS Server failed")
			}
		} else {
			_, _, _, err := net.cmdRunner.RunCommand("netmgr", "dns_servers", "--add", "--servers", dnsServer)

			if err != nil {
				return bosherr.WrapErrorf(err, "Setting DNS Server failed")
			}
		}
	}
	return nil

}

func (net photonosNetManager) buildInterfaces(networks boshsettings.Networks) ([]StaticInterfaceConfiguration, []DHCPInterfaceConfiguration, error) {
	interfacesByMacAddress, err := net.detectMacAddresses()
	if err != nil {
		return nil, nil, bosherr.WrapError(err, "Getting network interfaces")
	}

	staticInterfaceConfigurations, dhcpInterfaceConfigurations, err := net.interfaceConfigurationCreator.CreateInterfaceConfigurations(networks, interfacesByMacAddress)

	if err != nil {
		return nil, nil, bosherr.WrapError(err, "Creating interface configurations")
	}

	return staticInterfaceConfigurations, dhcpInterfaceConfigurations, nil
}

func (net photonosNetManager) broadcastIps(addresses []boship.InterfaceAddress, errCh chan error) {
	go func() {
		net.addressBroadcaster.BroadcastMACAddresses(addresses)
		if errCh != nil {
			errCh <- nil
		}
	}()
}

func (net photonosNetManager) restartNetworkingInterfaces() {
	net.logger.Debug(photonosNetManagerLogTag, "Restarting network interfaces")

	_, _, _, err := net.cmdRunner.RunCommand("systemctl", "restart", "systemd-networkd")
	if err != nil {
		net.logger.Error(photonosNetManagerLogTag, "Ignoring network restart failure: %s", err.Error())
	}
}

func (net photonosNetManager) detectMacAddresses() (map[string]string, error) {
	addresses := map[string]string{}

	stdout, _, _, err := net.cmdRunner.RunCommand("netmgr", "link_info", "--get")
	if err != nil {
		return addresses, bosherr.WrapError(err, "Getting link info from netmgr failed")
	}

	linkSlice := strings.Split(stdout, "\n")
	var macAddress string
	var interfaceInfo []string
	for i, linkInfo := range linkSlice {
		if i == 0 || i == len(linkSlice)-1 {
			continue
		}
                interfaceInfo = strings.Split(linkInfo, "\t")
		macAddress = strings.Trim(interfaceInfo[1], " ")
		interfaceName := strings.Trim(interfaceInfo[0], " ")
		addresses[macAddress] = interfaceName
	}

	return addresses, nil
}

func (net photonosNetManager) ifaceAddresses(staticConfigs []StaticInterfaceConfiguration, dhcpConfigs []DHCPInterfaceConfiguration) ([]boship.InterfaceAddress, []boship.InterfaceAddress) {
	staticAddresses := []boship.InterfaceAddress{}
	for _, iface := range staticConfigs {
		staticAddresses = append(staticAddresses, boship.NewSimpleInterfaceAddress(iface.Name, iface.Address))
	}
	dynamicAddresses := []boship.InterfaceAddress{}
	for _, iface := range dhcpConfigs {
		dynamicAddresses = append(dynamicAddresses, boship.NewResolvingInterfaceAddress(iface.Name, net.ipResolver))
	}

	return staticAddresses, dynamicAddresses
}
