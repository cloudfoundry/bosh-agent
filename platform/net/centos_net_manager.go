package net

import (
	"bytes"
	"path"
	"regexp"
	"strings"
	"text/template"
	"time"

	bosharp "github.com/cloudfoundry/bosh-agent/v2/platform/net/arp"
	boshdnsresolver "github.com/cloudfoundry/bosh-agent/v2/platform/net/dnsresolver"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const centosNetManagerLogTag = "centosNetManager"

type centosNetManager struct {
	fs                            boshsys.FileSystem
	cmdRunner                     boshsys.CmdRunner
	routesSearcher                RoutesSearcher //nolint:unused
	ipResolver                    boship.Resolver
	macAddressDetector            MACAddressDetector
	interfaceConfigurationCreator InterfaceConfigurationCreator
	interfaceAddrsProvider        boship.InterfaceAddressesProvider
	dnsResolver                   boshdnsresolver.DNSResolver
	addressBroadcaster            bosharp.AddressBroadcaster
	logger                        boshlog.Logger
}

func NewCentosNetManager(
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner,
	ipResolver boship.Resolver,
	macAddressDetector MACAddressDetector,
	interfaceConfigurationCreator InterfaceConfigurationCreator,
	interfaceAddrsProvider boship.InterfaceAddressesProvider,
	dnsResolver boshdnsresolver.DNSResolver,
	addressBroadcaster bosharp.AddressBroadcaster,
	logger boshlog.Logger,
) Manager {
	return centosNetManager{
		fs:                            fs,
		cmdRunner:                     cmdRunner,
		ipResolver:                    ipResolver,
		macAddressDetector:            macAddressDetector,
		interfaceConfigurationCreator: interfaceConfigurationCreator,
		interfaceAddrsProvider:        interfaceAddrsProvider,
		dnsResolver:                   dnsResolver,
		addressBroadcaster:            addressBroadcaster,
		logger:                        logger,
	}
}

func (net centosNetManager) GetConfiguredNetworkInterfaces() ([]string, error) {
	interfaces := []string{}

	interfacesByMacAddress, err := net.macAddressDetector.DetectMacAddresses()
	if err != nil {
		return interfaces, bosherr.WrapError(err, "Getting network interfaces")
	}

	for _, iface := range interfacesByMacAddress {
		if net.fs.FileExists(interfaceConfigurationFileCentos(iface)) {
			interfaces = append(interfaces, iface)
		}
	}

	return interfaces, nil
}

func (net centosNetManager) SetupIPv6(_ boshsettings.IPv6, _ <-chan struct{}) error { return nil }

func (net centosNetManager) SetupNetworking(networks boshsettings.Networks, mbus string, errCh chan error) error {
	// NOTE: Do not overwrite `/etc/resolv.conf` here, as it is controlled by Network Manager
	// This is an intentional asymmetry vs `ubuntu_net_manager.go`.
	// See commit 63548d43c69180b761d96b8e42a699e0762779e2.
	// See https://ma.ttias.be/centos-7-networkmanager-keeps-overwriting-etcresolv-conf/
	// See https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/8/html/configuring_and_managing_networking/manually-configuring-the-etc-resolv-conf-file_configuring-and-managing-networking
	// See https://wiseindy.com/blog/linux/how-to-set-dns-in-centos-rhel-7-prevent-network-manager-from-overwriting-etc-resolv-conf/

	nonVipNetworks := boshsettings.Networks{}
	for networkName, networkSettings := range networks {
		if networkSettings.IsVIP() {
			continue
		}
		nonVipNetworks[networkName] = networkSettings
	}

	staticConfigs, dhcpConfigs, err := net.buildInterfaces(nonVipNetworks)
	if err != nil {
		return err
	}

	dnsNetwork, _ := nonVipNetworks.DefaultNetworkFor("dns")
	dnsServers := dnsNetwork.DNS

	interfacesChanged, err := net.writeNetworkInterfaces(dhcpConfigs, staticConfigs, dnsServers)
	if err != nil {
		return bosherr.WrapError(err, "Writing network configuration")
	}

	dhcpChanged := false
	if len(dhcpConfigs) > 0 {
		dhcpChanged, err = net.writeDHCPConfiguration(dnsServers, dhcpConfigs)
		if err != nil {
			return err
		}
	}

	if interfacesChanged || dhcpChanged {
		net.restartNetworkingInterfaces()
	}

	staticAddresses, dynamicAddresses := net.ifaceAddresses(staticConfigs, dhcpConfigs)

	var staticAddressesWithoutVirtual []boship.InterfaceAddress
	r, err := regexp.Compile(`:\d+`)
	if err != nil {
		return bosherr.WrapError(err, "There is a problem with your regexp: ':\\d+'. That is used to skip validation of virtual interfaces(e.g., eth0:0, eth0:1)")
	}
	for _, addr := range staticAddresses {
		if r.MatchString(addr.GetInterfaceName()) {
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

	// NOTE: Do not overwrite `/etc/resolv.conf` here, as it is controlled by Network Manager
	// This is an intentional asymmetry vs `ubuntu_net_manager.go`.
	// See the comments at the top of this function for details.

	err = net.dnsResolver.Validate(dnsServers)
	if err != nil {
		return bosherr.WrapError(err, "Validating dns configuration")
	}

	go net.addressBroadcaster.BroadcastMACAddresses(append(staticAddressesWithoutVirtual, dynamicAddresses...))
	err = net.setupFirewall(mbus)
	if err != nil {
		return bosherr.WrapError(err, "Setting up Nats Firewall")
	}
	return nil
}

func (net centosNetManager) setupFirewall(mbus string) error {
	if mbus == "" {
		net.logger.Info("NetworkSetup", "Skipping adding Firewall for outgoing nats. Mbus url is empty")
		return nil
	}
	net.logger.Info("NetworkSetup", "Adding Firewall not implemented on")
	return nil
}

const centosDHCPIfcfgTemplate = `DEVICE={{ .Name }}
BOOTPROTO=dhcp
ONBOOT=yes
PEERDNS=yes
`

const centosStaticIfcfgTemplate = `DEVICE={{ .Name }}
BOOTPROTO=static
IPADDR={{ .Address }}
NETMASK={{ .Netmask }}
BROADCAST={{ .Broadcast }}{{if .IsDefaultForGateway}}
GATEWAY={{ .Gateway }}{{end}}
ONBOOT=yes
PEERDNS=no{{ range .DNSServers }}
DNS{{ .Index }}={{ .Address }}{{ end }}
`

type centosStaticIfcfg struct {
	*StaticInterfaceConfiguration
	DNSServers []dnsConfig
}

type dnsConfig struct {
	Index   int
	Address string
}

func newDNSConfigs(dnsServers []string) []dnsConfig {
	dnsConfigs := []dnsConfig{}
	for i := range dnsServers {
		dnsConfigs = append(dnsConfigs, dnsConfig{Index: i + 1, Address: dnsServers[i]})
	}
	return dnsConfigs
}

func interfaceConfigurationFileCentos(name string) string {
	return path.Join("/etc/sysconfig/network-scripts", "ifcfg-"+name)
}

func (net centosNetManager) writeIfcfgFile(name string, t *template.Template, config interface{}) (bool, error) {
	buffer := bytes.NewBuffer([]byte{})

	err := t.Execute(buffer, config)
	if err != nil {
		return false, bosherr.WrapErrorf(err, "Generating '%s' config from template", name)
	}

	filePath := interfaceConfigurationFileCentos(name)
	changed, err := net.fs.ConvergeFileContents(filePath, buffer.Bytes())
	if err != nil {
		return false, bosherr.WrapErrorf(err, "Writing config to '%s'", filePath)
	}

	return changed, nil
}

func (net centosNetManager) writeNetworkInterfaces(dhcpConfigs []DHCPInterfaceConfiguration, staticConfigs []StaticInterfaceConfiguration, dnsServers []string) (bool, error) {
	anyInterfaceChanged := false

	staticConfig := centosStaticIfcfg{}
	staticConfig.DNSServers = newDNSConfigs(dnsServers)
	staticTemplate := template.Must(template.New("ifcfg").Parse(centosStaticIfcfgTemplate))

	for i := range staticConfigs {
		staticConfig.StaticInterfaceConfiguration = &staticConfigs[i]

		changed, err := net.writeIfcfgFile(staticConfig.StaticInterfaceConfiguration.Name, staticTemplate, staticConfig)
		if err != nil {
			return false, bosherr.WrapError(err, "Writing static config")
		}

		anyInterfaceChanged = anyInterfaceChanged || changed
	}

	dhcpTemplate := template.Must(template.New("ifcfg").Parse(centosDHCPIfcfgTemplate))

	for i := range dhcpConfigs {
		config := &dhcpConfigs[i]

		changed, err := net.writeIfcfgFile(config.Name, dhcpTemplate, config)
		if err != nil {
			return false, bosherr.WrapError(err, "Writing dhcp config")
		}

		anyInterfaceChanged = anyInterfaceChanged || changed
	}

	return anyInterfaceChanged, nil
}

func (net centosNetManager) buildInterfaces(networks boshsettings.Networks) ([]StaticInterfaceConfiguration, []DHCPInterfaceConfiguration, error) {
	interfacesByMacAddress, err := net.macAddressDetector.DetectMacAddresses()
	if err != nil {
		return nil, nil, bosherr.WrapError(err, "Getting network interfaces")
	}

	staticConfigs, dhcpConfigs, err := net.interfaceConfigurationCreator.CreateInterfaceConfigurations(networks, interfacesByMacAddress)

	if err != nil {
		return nil, nil, bosherr.WrapError(err, "Creating interface configurations")
	}

	return staticConfigs, dhcpConfigs, nil
}

func (net centosNetManager) restartNetworkingInterfaces() {
	net.logger.Debug(centosNetManagerLogTag, "Restarting network interfaces")

	_, _, _, err := net.cmdRunner.RunCommand("service", "network", "restart")
	if err != nil {
		net.logger.Error(centosNetManagerLogTag, "Ignoring network restart failure: %s", err.Error())
	}
}

// DHCP Config file - /etc/dhcp3/dhclient.conf
const centosDHCPConfigTemplate = `# Generated by bosh-agent

option rfc3442-classless-static-routes code 121 = array of unsigned integer 8;

send host-name "<hostname>";

request subnet-mask, broadcast-address, time-offset, routers,
	domain-name, domain-name-servers, domain-search, host-name,
	netbios-name-servers, netbios-scope, interface-mtu,
	rfc3442-classless-static-routes, ntp-servers;
{{ if . }}
prepend domain-name-servers {{ . }};{{ end }}
`

func (net centosNetManager) writeDHCPConfiguration(dnsServers []string, dhcpConfigs []DHCPInterfaceConfiguration) (bool, error) {
	buffer := bytes.NewBuffer([]byte{})
	t := template.Must(template.New("dhcp-config").Parse(centosDHCPConfigTemplate))

	// Keep DNS servers in the order specified by the network
	// because they are added by a *single* DHCP's prepend command
	dnsServersList := strings.Join(dnsServers, ", ")
	err := t.Execute(buffer, dnsServersList)
	if err != nil {
		return false, bosherr.WrapError(err, "Generating config from template")
	}
	dhclientConfigFile := "/etc/dhcp/dhclient.conf"
	changed, err := net.fs.ConvergeFileContents(dhclientConfigFile, buffer.Bytes())

	if err != nil {
		return changed, bosherr.WrapErrorf(err, "Writing to %s", dhclientConfigFile)
	}

	for i := range dhcpConfigs {
		name := dhcpConfigs[i].Name
		interfaceDhclientConfigFile := path.Join("/etc/dhcp/", "dhclient-"+name+".conf")
		err = net.fs.Symlink(dhclientConfigFile, interfaceDhclientConfigFile)
		if err != nil {
			return changed, bosherr.WrapErrorf(err, "Symlinking '%s' to '%s'", interfaceDhclientConfigFile, dhclientConfigFile)
		}
	}

	return changed, nil
}

func (net centosNetManager) ifaceAddresses(staticConfigs []StaticInterfaceConfiguration, dhcpConfigs []DHCPInterfaceConfiguration) ([]boship.InterfaceAddress, []boship.InterfaceAddress) {
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
