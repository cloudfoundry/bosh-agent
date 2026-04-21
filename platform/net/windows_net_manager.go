package net

import (
	"fmt"
	gonet "net"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
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

	_, _, dnsServers, err := net.ComputeNetworkConfig(networks)
	if err != nil {
		return bosherr.WrapError(err, "Computing network configuration for dns")
	}

	_, _, _, err = net.runner.RunCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Service http")
	if err != nil {
		return bosherr.WrapError(err, "Starting HTTP service")
	}
	err = net.setupDNS(dnsServers)
	if err != nil {
		return err
	}

	if err := WriteLockFileForDNS(net.fs, net.dirProvider); err != nil {
		return bosherr.WrapError(err, "writing dns lockfile")
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
		if err := ValidateWindowsNetshInterfaceAlias(conf.Name); err != nil {
			return bosherr.WrapErrorf(err, "invalid network interface name %q", conf.Name)
		}

		gwArg := "none"
		if conf.IsDefaultForGateway && conf.Gateway != "" {
			canon, err := validateWindowsLiteralIPv4(conf.Gateway)
			if err != nil {
				return bosherr.WrapErrorf(err, "invalid gateway address %q", conf.Gateway)
			}
			gwArg = canon
		}

		args := []string{
			"interface", "ip", "set", "address",
			fmt.Sprintf("name=%s", conf.Name),
			"source=static",
			fmt.Sprintf("addr=%s", conf.Address),
			fmt.Sprintf("mask=%s", conf.Netmask),
			fmt.Sprintf("gateway=%s", gwArg),
		}
		if gwArg != "none" {
			args = append(args, "gwmetric=1")
		}

		_, _, _, err := net.runner.RunCommand("netsh", args...)
		if err != nil {
			return bosherr.WrapErrorf(err, "Configuring interface %q", conf.Name)
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

	var script string
	if len(dnsServers) > 0 {
		net.logger.Info(net.logTag, "Setting DNS servers: %v", dnsServers)
		var parts []string
		for _, d := range dnsServers {
			canon, err := validateWindowsLiteralIPv4(d)
			if err != nil {
				return bosherr.WrapErrorf(err, "invalid DNS server IP %q", strings.TrimSpace(d))
			}
			parts = append(parts, "'"+canon+"'")
		}
		script = fmt.Sprintf(
			`$dns=@(%s); Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ServerAddresses $dns }`,
			strings.Join(parts, ","),
		)
	} else {
		net.logger.Info(net.logTag, "Resetting DNS servers")
		script = `Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ResetServerAddresses }`
	}

	_, _, _, err := net.runner.RunCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	if err != nil {
		return bosherr.WrapError(err, "Setting DNS servers")
	}
	return nil
}

// validateWindowsLiteralIPv4 parses a host or DNS server address for netsh / Set-DnsClientServerAddress (IPv4 literals only).
func validateWindowsLiteralIPv4(s string) (string, error) {
	trimmed := strings.TrimSpace(s)
	ip := gonet.ParseIP(trimmed)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address %q", trimmed)
	}
	if ip.To4() == nil {
		return "", fmt.Errorf("invalid IPv4 address %q (IPv6 is not supported for this configuration path)", trimmed)
	}
	return ip.String(), nil
}

func ValidateWindowsNetshInterfaceAlias(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("network interface name is empty")
	}
	if strings.ContainsAny(name, "\"\r\n") {
		return fmt.Errorf("network interface name must not contain double quotes or newlines")
	}
	return nil
}
