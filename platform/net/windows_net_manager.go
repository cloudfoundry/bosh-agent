package net

import (
	"fmt"
	gonet "net"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type WindowsNetManager struct {
	runner                        boshsys.CmdRunner
	interfaceConfigurationCreator InterfaceConfigurationCreator
	macAddressDetector            MACAddressDetector
	logTag                        string
	logger                        boshlog.Logger
	clock                         clock.Clock
	fs                            boshsys.FileSystem
	dirProvider                   boshdirs.Provider
}

func NewWindowsNetManager(
	runner boshsys.CmdRunner,
	interfaceConfigurationCreator InterfaceConfigurationCreator,
	macAddressDetector MACAddressDetector,
	logger boshlog.Logger,
	clock clock.Clock,
	fs boshsys.FileSystem,
	dirProvider boshdirs.Provider,
) Manager {
	return WindowsNetManager{
		runner:                        runner,
		interfaceConfigurationCreator: interfaceConfigurationCreator,
		macAddressDetector:            macAddressDetector,
		logTag:                        "WindowsNetManager",
		logger:                        logger,
		clock:                         clock,
		fs:                            fs,
		dirProvider:                   dirProvider,
	}
}

const (
	SetDNSTemplate = `
[array]$interfaces = Get-DNSClientServerAddress
$dns = @("%s")
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ServerAddresses ($dns -join ",")
}
`

	ResetDNSTemplate = `
[array]$interfaces = Get-DNSClientServerAddress
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ResetServerAddresses
}
`

	NicSettingsTemplate = `
netsh interface ip set address %q static %s %s %s
`
)

// GetConfiguredNetworkInterfaces returns all of the network interfaces if a
// previous call to SetupNetworking succeeded as indicated by the presence of
// a file ("configured_interfaces.txt").
//
// A file is used because there is no good way to determine if network
// interfaces are configured on Windows and SetupNetworking may be called
// during bootstrap so it is possible the agent will have restarted since
// it the last call.
//
// We return all of the network interfaces as we configure DNS for all of the
// network interfaces.  Apart from DNS, the returned network interfaces may
// not have been configured.
func (net WindowsNetManager) GetConfiguredNetworkInterfaces() ([]string, error) {
	net.logger.Info(net.logTag, "Getting Configured Network Interfaces...")

	if !LockFileExistsForConfiguredInterfaces(net.dirProvider) {
		net.logger.Info(net.logTag, "No network interfaces file")
		if err := writeLockFileForConfiguredInterfaces(net.logger, net.logTag, net.dirProvider, net.fs); err != nil {
			return nil, bosherr.WrapError(err, "Writing configured network interfaces")
		}

		initialNetworks := boshsettings.Networks{
			"eth0": {
				Type: boshsettings.NetworkTypeDynamic,
			},
		}
		if err := net.setupNetworkInterfaces(initialNetworks); err != nil {
			return nil, bosherr.WrapError(err, "Setting up windows DHCP network")
		}
	}

	net.logger.Info(net.logTag, "Found network interfaces file")

	ifs, err := gonet.Interfaces()
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Getting network interfaces: %s", err)
	}
	names := make([]string, 0, len(ifs))
	for _, f := range ifs {
		names = append(names, f.Name)
	}
	return names, nil
}

func (net WindowsNetManager) setupNetworkInterfaces(networks boshsettings.Networks) error {
	staticConfigs, _, _, err := net.ComputeNetworkConfig(networks)
	if err != nil {
		return bosherr.WrapError(err, "Computing network configuration")
	}

	if err := net.setupInterfaces(staticConfigs); err != nil {
		return err
	}

	net.clock.Sleep(5 * time.Second)
	return nil
}

func (net WindowsNetManager) SetupNetworking(networks boshsettings.Networks, mbus string, errCh chan error) error {
	if err := net.setupNetworkInterfaces(networks); err != nil {
		return bosherr.WrapError(err, "setting up network interfaces")
	}
	if err := net.setupFirewall(mbus); err != nil {
		return bosherr.WrapError(err, "Setting up Nats Firewall")
	}
	if LockFileExistsForDNS(net.fs, net.dirProvider) {
		return nil
	}

	if err := WriteLockFileForDNS(net.fs, net.dirProvider); err != nil {
		return bosherr.WrapError(err, "writing dns lockfile")
	}

	_, _, dnsServers, err := net.ComputeNetworkConfig(networks)
	if err != nil {
		return bosherr.WrapError(err, "Computing network configuration for dns")
	}

	_, _, _, err = net.runner.RunCommand("powershell", "-Command", "Start-Service http")
	if err != nil {
		return bosherr.WrapError(err, "Starting HTTP service")
	}
	err = net.setupDNS(dnsServers)
	if err != nil {
		return err
	}

	return nil
}
func (net WindowsNetManager) setupFirewall(mbus string) error {
	if mbus == "" {
		net.logger.Info("NetworkSetup", "Skipping adding Firewall for outgoing nats. Mbus url is empty")
		return nil
	}
	net.logger.Info("NetworkSetup", "Adding Firewall")
	return SetupNatsFirewall(mbus)
}
func (net WindowsNetManager) ComputeNetworkConfig(networks boshsettings.Networks) (
	[]StaticInterfaceConfiguration,
	[]DHCPInterfaceConfiguration,
	[]string,
	error,
) {
	nonVipNetworks := boshsettings.Networks{}
	for networkName, networkSettings := range networks {
		if networkSettings.IsVIP() {
			continue
		}
		nonVipNetworks[networkName] = networkSettings
	}

	staticConfigs, dhcpConfigs, err := net.buildInterfaces(nonVipNetworks)
	if err != nil {
		return nil, nil, nil, err
	}

	dnsNetwork, _ := nonVipNetworks.DefaultNetworkFor("dns")
	dnsServers := dnsNetwork.DNS
	return staticConfigs, dhcpConfigs, dnsServers, nil
}

func (net WindowsNetManager) SetupIPv6(_ boshsettings.IPv6, _ <-chan struct{}) error { return nil }

func (net WindowsNetManager) setupInterfaces(staticConfigs []StaticInterfaceConfiguration) error {
	for _, conf := range staticConfigs {
		var gateway string
		if conf.IsDefaultForGateway {
			gateway = conf.Gateway
		}

		content := fmt.Sprintf(NicSettingsTemplate, conf.Name, conf.Address, conf.Netmask, gateway)

		_, _, _, err := net.runner.RunCommand("powershell", "-Command", content)
		if err != nil {
			return bosherr.WrapError(err, "Configuring interface")
		}
	}
	return nil
}

func (net WindowsNetManager) buildInterfaces(networks boshsettings.Networks) (
	[]StaticInterfaceConfiguration,
	[]DHCPInterfaceConfiguration,
	error,
) {
	interfacesByMacAddress, err := net.macAddressDetector.DetectMacAddresses()
	if err != nil {
		return nil, nil, bosherr.WrapError(err, "Getting network interfaces")
	}

	staticConfigs, dhcpConfigs, err := net.interfaceConfigurationCreator.CreateInterfaceConfigurations(
		networks, interfacesByMacAddress)
	if err != nil {
		return nil, nil, bosherr.WrapError(err, "Creating interface configurations")
	}

	return staticConfigs, dhcpConfigs, nil
}

func (net WindowsNetManager) setupDNS(dnsServers []string) error {
	net.logger.Info(net.logTag, "Setting up DNS...")

	var content string
	if len(dnsServers) > 0 {
		net.logger.Info(net.logTag, "Setting DNS servers: %v", dnsServers)
		content = fmt.Sprintf(SetDNSTemplate, strings.Join(dnsServers, `","`))
	} else {
		net.logger.Info(net.logTag, "Resetting DNS servers")
		content = ResetDNSTemplate
	}

	_, _, _, err := net.runner.RunCommand("powershell", "-Command", content)
	if err != nil {
		return bosherr.WrapError(err, "Setting DNS servers")
	}
	return nil
}
