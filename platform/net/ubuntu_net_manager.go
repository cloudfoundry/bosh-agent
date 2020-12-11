package net

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	bosharp "github.com/cloudfoundry/bosh-agent/platform/net/arp"
	boship "github.com/cloudfoundry/bosh-agent/platform/net/ip"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	ini "github.com/cloudfoundry/bosh-agent/ini"
)

const UbuntuNetManagerLogTag = "UbuntuNetManager"

type UbuntuNetManager struct {
	cmdRunner                     boshsys.CmdRunner
	fs                            boshsys.FileSystem
	ipResolver                    boship.Resolver
	macAddressDetector            MACAddressDetector
	interfaceConfigurationCreator InterfaceConfigurationCreator
	interfaceAddrsProvider        boship.InterfaceAddressesProvider
	dnsValidator                  DNSValidator
	addressBroadcaster            bosharp.AddressBroadcaster
	kernelIPv6                    KernelIPv6
	logger                        boshlog.Logger
}

func NewUbuntuNetManager(
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner,
	ipResolver boship.Resolver,
	macAddressDetector MACAddressDetector,
	interfaceConfigurationCreator InterfaceConfigurationCreator,
	interfaceAddrsProvider boship.InterfaceAddressesProvider,
	dnsValidator DNSValidator,
	addressBroadcaster bosharp.AddressBroadcaster,
	kernelIPv6 KernelIPv6,
	logger boshlog.Logger,
) Manager {
	return UbuntuNetManager{
		cmdRunner:                     cmdRunner,
		fs:                            fs,
		ipResolver:                    ipResolver,
		macAddressDetector:            macAddressDetector,
		interfaceConfigurationCreator: interfaceConfigurationCreator,
		interfaceAddrsProvider:        interfaceAddrsProvider,
		dnsValidator:                  dnsValidator,
		addressBroadcaster:            addressBroadcaster,
		kernelIPv6:                    kernelIPv6,
		logger:                        logger,
	}
}

func (net UbuntuNetManager) GetConfiguredNetworkInterfaces() ([]string, error) {
	interfaces := []string{}

	interfacesByMacAddress, err := net.macAddressDetector.DetectMacAddresses()
	if err != nil {
		return interfaces, bosherr.WrapError(err, "Getting network interfaces")
	}

	for _, iface := range interfacesByMacAddress {
		if net.fs.FileExists(interfaceConfigurationFile(iface)) {
			interfaces = append(interfaces, iface)
		}
	}

	return interfaces, nil
}

func (net UbuntuNetManager) SetupIPv6(config boshsettings.IPv6, stopCh <-chan struct{}) error {
	if config.Enable {
		return net.kernelIPv6.Enable(stopCh)
	}
	return nil
}

func (net UbuntuNetManager) SetupNetworking(networks boshsettings.Networks, errCh chan error) error {
	if networks.IsPreconfigured() {
		// Note in this case IPs are not broadcast
		dnsNetwork, _ := networks.DefaultNetworkFor("dns")
		return net.writeResolvConf(dnsNetwork.DNS)
	}

	staticConfigs, dhcpConfigs, dnsServers, err := net.ComputeNetworkConfig(networks)
	if err != nil {
		return bosherr.WrapError(err, "Computing network configuration")
	}

	if StaticInterfaceConfigurations(staticConfigs).HasVersion6() {
		err := net.kernelIPv6.Enable(make(chan struct{}))
		if err != nil {
			return bosherr.WrapError(err, "Enabling IPv6 in kernel")
		}
	}

	changed, err := net.writeNetConfigs(dhcpConfigs, staticConfigs, dnsServers, boshsys.ConvergeFileContentsOpts{})
	if err != nil {
		return bosherr.WrapError(err, "Updating network configs")
	}

	if changed {
		err = net.removeDhcpDNSConfiguration()
		if err != nil {
			return err
		}

		err := net.restartNetworking()
		if err != nil {
			return bosherr.WrapError(err, "Failure restarting networking")
		}
	}

	staticAddresses, dynamicAddresses := net.ifaceAddresses(staticConfigs, dhcpConfigs)

	var staticAddressesWithoutVirtual []boship.InterfaceAddress
	r, err := regexp.Compile(`:\d+`)
	if err != nil {
		return bosherr.WrapError(err, "There is a problem with your regexp: ':\\d+'. That is used to skip validation of virtual interfaces(e.g., eth0:0, eth0:1)")
	}
	for _, addr := range staticAddresses {
		if r.MatchString(addr.GetInterfaceName()) == true {
			continue
		} else {
			staticAddressesWithoutVirtual = append(staticAddressesWithoutVirtual, addr)
		}
	}

	interfaceAddressesValidator := boship.NewInterfaceAddressesValidator(net.interfaceAddrsProvider, staticAddressesWithoutVirtual)
	retryIPValidator := boshretry.NewAttemptRetryStrategy(
		10,
		time.Second,
		interfaceAddressesValidator,
		net.logger,
	)
	err = retryIPValidator.Try()
	if err != nil {
		return bosherr.WrapError(err, "Validating static network configuration")
	}

	err = net.writeResolvConf(dnsServers)
	if err != nil {
		return bosherr.WrapError(err, "Updating /etc/resolv.conf")
	}

	err = net.dnsValidator.Validate(dnsServers)
	if err != nil {
		return bosherr.WrapError(err, "Validating dns configuration")
	}

	go net.addressBroadcaster.BroadcastMACAddresses(append(staticAddressesWithoutVirtual, dynamicAddresses...))

	return nil
}

func (net UbuntuNetManager) ComputeNetworkConfig(networks boshsettings.Networks) ([]StaticInterfaceConfiguration, []DHCPInterfaceConfiguration, []string, error) {
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

	staticConfigs, err = net.collapseVirtualInterfaces(staticConfigs)
	if err != nil {
		return nil, nil, nil, err
	}

	dnsNetwork, _ := nonVipNetworks.DefaultNetworkFor("dns")
	dnsServers := dnsNetwork.DNS
	return staticConfigs, dhcpConfigs, dnsServers, nil
}

func (net UbuntuNetManager) collapseVirtualInterfaces(staticConfigs []StaticInterfaceConfiguration) ([]StaticInterfaceConfiguration, error) {
	configs := []StaticInterfaceConfiguration{}

	// collect any virtual interfaces
	virtualInterfacesByDevice := map[string][]VirtualInterface{}
	for _, config := range staticConfigs {
		if strings.Contains(config.Name, ":") {
			ifaceName := strings.Split(config.Name, ":")[0]

			if _, ok := virtualInterfacesByDevice[ifaceName]; !ok {
				virtualInterfacesByDevice[ifaceName] = []VirtualInterface{}
			}
			cidr, err := config.CIDR()
			if err != nil {
				return nil, err
			}

			virtualInterfacesByDevice[ifaceName] = append(
				virtualInterfacesByDevice[ifaceName],
				VirtualInterface{Label: config.Name, Address: fmt.Sprintf("%s/%s", config.Address, cidr)},
			)
		}
	}

	// keep non-virtual interfaces, and append if found
	for _, config := range staticConfigs {
		if !strings.Contains(config.Name, ":") {
			if virtualInterfaces, ok := virtualInterfacesByDevice[config.Name]; ok {
				config.VirtualInterfaces = virtualInterfaces
			}

			configs = append(configs, config)
		}
	}

	return configs, nil
}

func (net UbuntuNetManager) writeNetConfigs(
	dhcpConfigs DHCPInterfaceConfigurations,
	staticConfigs StaticInterfaceConfigurations,
	dnsServers []string,
	opts boshsys.ConvergeFileContentsOpts) (bool, error) {

	interfacesChanged, err := net.writeNetworkInterfaces(dhcpConfigs, staticConfigs, dnsServers, opts)
	if err != nil {
		return false, bosherr.WrapError(err, "Writing network configuration")
	}

	dhcpChanged := false

	if len(dhcpConfigs) > 0 {
		dhcpChanged, err = net.writeDHCPConfiguration(dnsServers, opts)
		if err != nil {
			return false, err
		}
	}

	return (interfacesChanged || dhcpChanged), nil
}

func (net UbuntuNetManager) removeDhcpDNSConfiguration() error {
	// Removing dhcp configuration from /etc/network/interfaces
	// and restarting network does not stop dhclient if dhcp
	// is no longer needed. See https://bugs.launchpad.net/ubuntu/+source/dhcp3/+bug/38140
	_, _, _, err := net.cmdRunner.RunCommand("pkill", "dhclient")
	if err != nil {
		net.logger.Error(UbuntuNetManagerLogTag, "Ignoring failure calling 'pkill dhclient': %s", err)
	}

	interfacesByMacAddress, err := net.macAddressDetector.DetectMacAddresses()
	if err != nil {
		return err
	}

	for _, ifaceName := range interfacesByMacAddress {
		// Explicitly delete the resolvconf record about given iface
		// It seems to hold on to old dhclient records after dhcp configuration
		// is removed from /etc/network/interfaces.
		_, _, _, err = net.cmdRunner.RunCommand("resolvconf", "-d", ifaceName+".dhclient")
		if err != nil {
			net.logger.Error(UbuntuNetManagerLogTag, "Ignoring failure calling 'resolvconf -d %s.dhclient': %s", ifaceName, err)
		}
	}

	return nil
}

func (net UbuntuNetManager) buildInterfaces(networks boshsettings.Networks) ([]StaticInterfaceConfiguration, []DHCPInterfaceConfiguration, error) {
	interfacesByMacAddress, err := net.macAddressDetector.DetectMacAddresses()
	if err != nil {
		return nil, nil, bosherr.WrapError(err, "Getting network interfaces")
	}

	// if len(interfacesByMacAddress) == 0 {
	// 	return nil, nil, bosherr.Error("No network interfaces found")
	// }

	staticConfigs, dhcpConfigs, err := net.interfaceConfigurationCreator.CreateInterfaceConfigurations(networks, interfacesByMacAddress)
	if err != nil {
		return nil, nil, bosherr.WrapError(err, "Creating interface configurations")
	}

	return staticConfigs, dhcpConfigs, nil
}

func (net UbuntuNetManager) ifaceAddresses(staticConfigs []StaticInterfaceConfiguration, dhcpConfigs []DHCPInterfaceConfiguration) ([]boship.InterfaceAddress, []boship.InterfaceAddress) {
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

func (net UbuntuNetManager) restartNetworking() error {
	_, _, _, err := net.cmdRunner.RunCommand("/var/vcap/bosh/bin/restart_networking")
	if err != nil {
		return err
	}
	return nil
}

func (net UbuntuNetManager) writeDHCPConfiguration(dnsServers []string, opts boshsys.ConvergeFileContentsOpts) (bool, error) {
	buffer := bytes.NewBuffer([]byte{})
	t := template.Must(template.New("dhcp-config").Parse(dhclientConfTemplate))

	// Keep DNS servers in the order specified by the network
	// because they are added by a *single* DHCP's prepend command
	dnsServersList := strings.Join(dnsServers, ", ")
	err := t.Execute(buffer, dnsServersList)
	if err != nil {
		return false, bosherr.WrapError(err, "Generating config from template")
	}

	dhclientConfigFile := "/etc/dhcp/dhclient.conf"

	changed, err := net.fs.ConvergeFileContents(dhclientConfigFile, buffer.Bytes(), opts)
	if err != nil {
		return changed, bosherr.WrapErrorf(err, "Writing to %s", dhclientConfigFile)
	}

	return changed, nil
}

func (net UbuntuNetManager) updateConfiguration(name, templateDefinition string, templateConfiguration interface{}, opts boshsys.ConvergeFileContentsOpts) (bool, error) {
	interfaceFile := interfaceConfigurationFile(name)
	buffer := bytes.NewBuffer([]byte{})
	templateFuncs := template.FuncMap{
		"NetmaskToCIDR": boshsettings.NetmaskToCIDR,
	}

	t := template.Must(template.New(name).Funcs(templateFuncs).Parse(templateDefinition))

	err := t.Execute(buffer, templateConfiguration)
	if err != nil {
		return false, bosherr.WrapError(err, fmt.Sprintf("Generating config from template %s", name))
	}

	net.logger.Error(UbuntuNetManagerLogTag, "Updating %s configuration with contents: %s", name, buffer.Bytes())
	return net.fs.ConvergeFileContents(
		interfaceFile,
		buffer.Bytes(),
		opts,
	)
}

const systemdNetworkFolder = "/etc/systemd/network"

func interfaceConfigurationFile(name string) string {
	interfaceBasename := fmt.Sprintf("09_%s.network", name)
	return filepath.Join(systemdNetworkFolder, interfaceBasename)
}

func (net UbuntuNetManager) writeNetworkInterfaces(
	dhcpConfigs DHCPInterfaceConfigurations,
	staticConfigs StaticInterfaceConfigurations,
	dnsServers []string,
	opts boshsys.ConvergeFileContentsOpts) (bool, error) {

	type networkInterfaceConfig struct {
		DNSServers        []string
		InterfaceConfig   interface{}
		HasDNSNameServers bool
	}

	sort.Stable(dhcpConfigs)
	sort.Stable(staticConfigs)

	staleNetworkConfigFiles := make(map[string]bool)
	err := net.fs.Walk(filepath.Join(systemdNetworkFolder), func(match string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		staleNetworkConfigFiles[match] = true
		return nil
	})

	if err != nil {
		return false, err
	}

	anyChanged := false
	for _, dynamicAddressConfiguration := range dhcpConfigs {
		changed, err := net.writeDynamicInterfaceConfiguration(dynamicAddressConfiguration, dnsServers, opts)
		if err != nil {
			return false, bosherr.WrapError(err, fmt.Sprintf("Updating network configuration for %s", dynamicAddressConfiguration.Name))
		}

		newNetworkFile := interfaceConfigurationFile(dynamicAddressConfiguration.Name)
		if _, ok := staleNetworkConfigFiles[newNetworkFile]; ok {
			staleNetworkConfigFiles[newNetworkFile] = false
		}

		anyChanged = anyChanged || changed
	}

	for _, staticAddressConfiguration := range staticConfigs {
		changed, err := net.writeStaticInterfaceConfiguration(staticAddressConfiguration, dnsServers, opts)
		if err != nil {
			return false, bosherr.WrapError(err, fmt.Sprintf("Updating network configuration for %s", staticAddressConfiguration.Name))
		}

		newNetworkFile := interfaceConfigurationFile(staticAddressConfiguration.Name)
		if _, ok := staleNetworkConfigFiles[newNetworkFile]; ok {
			staleNetworkConfigFiles[newNetworkFile] = false
		}

		anyChanged = anyChanged || changed
	}

	for networkFile, isStale := range staleNetworkConfigFiles {
		if networkFile == systemdNetworkFolder {
			continue
		}
		if isStale {
			err := net.fs.RemoveAll(networkFile)
			if err != nil {
				return false, err
			}
			anyChanged = true
		}
	}
	return anyChanged, nil
}

func (net UbuntuNetManager) ifaceNames(dhcpConfigs DHCPInterfaceConfigurations, staticConfigs StaticInterfaceConfigurations) []string {
	ifaceNames := []string{}
	for _, config := range dhcpConfigs {
		ifaceNames = append(ifaceNames, config.Name)
	}
	for _, config := range staticConfigs {
		ifaceNames = append(ifaceNames, config.Name)
	}
	return ifaceNames
}

func (net UbuntuNetManager) writeResolvConf(dnsServers []string) error {
	buffer := bytes.NewBuffer([]byte{})

	t := template.Must(template.New("resolv-conf").Parse(resolvConfTemplate))

	type dnsConfigArg struct {
		DNSServers []string
	}

	dnsServersArg := dnsConfigArg{dnsServers}

	err := t.Execute(buffer, dnsServersArg)
	if err != nil {
		return bosherr.WrapError(err, "Generating DNS config from template")
	}

	if len(dnsServers) > 0 {
		// Write out base so that releases may overwrite head
		err = net.fs.WriteFile("/etc/resolvconf/resolv.conf.d/base", buffer.Bytes())
		if err != nil {
			return bosherr.WrapError(err, "Writing to /etc/resolvconf/resolv.conf.d/base")
		}
	} else {
		// For the first time before resolv.conf is symlinked to /run/...
		// inherit possibly configured resolv.conf

		targetPath, err := net.fs.ReadAndFollowLink("/etc/resolv.conf")
		if err != nil {
			return bosherr.WrapError(err, "Reading /etc/resolv.conf symlink")
		}

		expectedPath, err := filepath.Abs("/etc/resolv.conf")
		if err != nil {
			return bosherr.WrapError(err, "Resolving path to native OS")
		}
		if targetPath == expectedPath {
			err := net.fs.CopyFile("/etc/resolv.conf", "/etc/resolvconf/resolv.conf.d/base")
			if err != nil {
				return bosherr.WrapError(err, "Copying /etc/resolv.conf for backwards compat")
			}
		}
	}

	err = net.fs.Symlink("/run/resolvconf/resolv.conf", "/etc/resolv.conf")
	if err != nil {
		return bosherr.WrapError(err, "Setting up /etc/resolv.conf symlink")
	}

	_, _, _, err = net.cmdRunner.RunCommand("resolvconf", "-u")
	if err != nil {
		return bosherr.WrapError(err, "Updating resolvconf")
	}

	return nil
}

func (net UbuntuNetManager) writeStaticInterfaceConfiguration(config StaticInterfaceConfiguration, dnsServers []string, opts boshsys.ConvergeFileContentsOpts) (bool, error) {
	var err error
	configPath := interfaceConfigurationFile(config.Name)

	cidr, err := config.CIDR()
	if err != nil {
		return false, err
	}

	file := ini.Empty()
	file.Comment = "# Generated by bosh-agent"

	// Match Section
	matchSection := &ini.Section{Name: "Match"}
	matchSection.AddKey("Name", config.Name)
	file.AppendSection(matchSection)

	// Address Section
	addressSection := &ini.Section{Name: "Address"}
	addressSection.AddKey("Address", fmt.Sprintf("%s/%s", config.Address, cidr))
	if config.IsDefaultForGateway && !config.IsVersion6() {
		addressSection.AddKey("Broadcast", config.Broadcast)
	}
	file.AppendSection(addressSection)

	// Virtual Interfaces
	for _, virtualInterface := range config.VirtualInterfaces {
		addressSection := &ini.Section{Name: "Address"}
		addressSection.AddKey("Label", virtualInterface.Label)
		addressSection.AddKey("Address", virtualInterface.Address)
		file.AppendSection(addressSection)
	}

	// Network Section
	networkSection := &ini.Section{Name: "Network"}
	if config.IsDefaultForGateway {
		networkSection.AddKey("Gateway", config.Gateway)
	}

	if config.IsVersion6() {
		networkSection.AddKey("IPv6AcceptRA", "true")
	}

	for _, dnsServer := range dnsServers {
		networkSection.AddKey("DNS", dnsServer)
	}
	file.AppendSection(networkSection)

	// Route Sections
	for _, postUpRoute := range config.PostUpRoutes {
		routeSection := &ini.Section{Name: "Route"}
		postUpRouteCidr, err := boshsettings.NetmaskToCIDR(postUpRoute.Netmask, config.IsVersion6())
		if err != nil {
			return false, err
		}

		routeSection.AddKey("Destination", fmt.Sprintf("%s/%s", postUpRoute.Destination, postUpRouteCidr))
		routeSection.AddKey("Gateway", postUpRoute.Gateway)

		file.AppendSection(routeSection)
	}

	buffer := bytes.NewBuffer(nil)
	_, err = file.WriteTo(buffer)
	if err != nil {
		return false, err
	}

	return net.fs.ConvergeFileContents(configPath, buffer.Bytes(), opts)
}

func (net UbuntuNetManager) writeDynamicInterfaceConfiguration(config DHCPInterfaceConfiguration, dnsServers []string, opts boshsys.ConvergeFileContentsOpts) (bool, error) {
	if config.Address == "" {
		return false, nil
	}

	var err error
	configPath := interfaceConfigurationFile(config.Name)

	file := ini.Empty()
	file.Comment = "# Generated by bosh-agent"

	// Match Section
	matchSection := &ini.Section{Name: "Match"}
	matchSection.AddKey("Name", config.Name)
	file.AppendSection(matchSection)

	// Network Section
	networkSection := &ini.Section{Name: "Network"}
	networkSection.AddKey("DHCP", "yes")
	if config.IsVersion6() {
		networkSection.AddKey("IPv6AcceptRA", "true")
	}

	for _, dnsServer := range dnsServers {
		networkSection.AddKey("DNS", dnsServer)
	}
	file.AppendSection(networkSection)

	// DHCP Section
	dhcpSection := &ini.Section{Name: "DHCP"}
	dhcpSection.AddKey("UseDomains", "yes")
	file.AppendSection(dhcpSection)

	// Route Sections
	for _, postUpRoute := range config.PostUpRoutes {
		routeSection := &ini.Section{Name: "Route"}
		postUpRouteCidr, err := boshsettings.NetmaskToCIDR(postUpRoute.Netmask, config.IsVersion6())
		if err != nil {
			return false, err
		}

		routeSection.AddKey("Destination", fmt.Sprintf("%s/%s", postUpRoute.Destination, postUpRouteCidr))
		routeSection.AddKey("Gateway", postUpRoute.Gateway)

		file.AppendSection(routeSection)
	}

	buffer := bytes.NewBuffer(nil)
	_, err = file.WriteTo(buffer)
	if err != nil {
		return false, err
	}

	return net.fs.ConvergeFileContents(configPath, buffer.Bytes(), opts)
}

// DHCP Config file - /etc/dhcp/dhclient.conf
// Ubuntu 14.04 accepts several DNS as a list in a single prepend directive
const dhclientConfTemplate = `# Generated by bosh-agent

option rfc3442-classless-static-routes code 121 = array of unsigned integer 8;

send host-name = gethostname();

request subnet-mask, broadcast-address, time-offset, routers,
	domain-name, domain-name-servers, domain-search, host-name,
	netbios-name-servers, netbios-scope, interface-mtu,
	rfc3442-classless-static-routes, ntp-servers;
{{ if . }}
prepend domain-name-servers {{ . }};{{ end }}
`
const resolvConfTemplate = `# Generated by bosh-agent
{{ range .DNSServers }}nameserver {{ . }}
{{ end }}`
