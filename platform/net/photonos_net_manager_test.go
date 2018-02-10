package net_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net"
	fakearp "github.com/cloudfoundry/bosh-agent/platform/net/arp/fakes"
	boship "github.com/cloudfoundry/bosh-agent/platform/net/ip"
	fakeip "github.com/cloudfoundry/bosh-agent/platform/net/ip/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("photonosNetManager", describePhotonosNetManager)

func describePhotonosNetManager() {
	var (
		fs                            *fakesys.FakeFileSystem
		cmdRunner                     *fakesys.FakeCmdRunner
		ipResolver                    *fakeip.FakeResolver
		interfaceAddrsProvider        *fakeip.FakeInterfaceAddressesProvider
		addressBroadcaster            *fakearp.FakeAddressBroadcaster
		netManager                    Manager
		interfaceConfigurationCreator InterfaceConfigurationCreator
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		cmdRunner = fakesys.NewFakeCmdRunner()
		ipResolver = &fakeip.FakeResolver{}
		logger := boshlog.NewLogger(boshlog.LevelNone)
		interfaceConfigurationCreator = NewInterfaceConfigurationCreator(logger)
		interfaceAddrsProvider = &fakeip.FakeInterfaceAddressesProvider{}
		interfaceAddrsValidator := boship.NewInterfaceAddressesValidator(interfaceAddrsProvider)
		dnsValidator := NewDNSValidator(fs)
		addressBroadcaster = &fakearp.FakeAddressBroadcaster{}
		netManager = NewPhotonosNetManager(
			fs,
			cmdRunner,
			ipResolver,
			interfaceConfigurationCreator,
			interfaceAddrsValidator,
			dnsValidator,
			addressBroadcaster,
			logger,
		)
	})

	writeOutputLine := func(iface string, macAddress string) string {
		interfacePath := fmt.Sprintf("%s\t%s\tauto      	1500      	up                     \n", iface, macAddress)

		return interfacePath
	}

	Describe("SetupNetworking", func() {
		var (
			dhcpNetwork                           boshsettings.Network
			staticNetwork                         boshsettings.Network
			expectedNetworkConfigurationForStatic string
			expectedNetworkConfigurationForDHCP   string
			expectedResolveConf                   string
		)

		BeforeEach(func() {
			dhcpNetwork = boshsettings.Network{
				Type:    "dynamic",
				Default: []string{"dns"},
				DNS:     []string{"8.8.8.8", "9.9.9.9"},
				Mac:     "fake-dhcp-mac-address",
			}
			staticNetwork = boshsettings.Network{
				Type:    "manual",
				IP:      "1.2.3.4",
				Netmask: "255.255.255.0",
				Gateway: "3.4.5.6",
				Mac:     "fake-static-mac-address",
			}
			interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("ethstatic", "1.2.3.4"),
			}
			fs.WriteFileString("/etc/resolv.conf", `
nameserver 8.8.8.8
nameserver 9.9.9.9
`)

			expectedResolveConf = `
[Resolve]
LLMNR=false
DNS=8.8.8.8 9.9.9.9
`

			expectedNetworkConfigurationForStatic = `
[Match]
Name=ethstatic

[Network]
DHCP=no
IPv6AcceptRA=no
Address=1.2.3.4/24
Gateway=3.4.5.6
`

			expectedNetworkConfigurationForDHCP = `
[Match]
Name=ethdhcp

[Network]
DHCP=ipv4
IPv6AcceptRA=no
`

		})

		stubInterfacesWithVirtual := func(physicalInterfaces map[string]boshsettings.Network, virtualInterfaces []string) {
			interfacePaths := []string{}

			netmgrGetLinkInfo := "Name      	MacAddress       	Mode      	MTU       	State    \n"
			for iface, networkSettings := range physicalInterfaces {
				netmgrGetLinkInfo = netmgrGetLinkInfo + writeOutputLine(iface, networkSettings.Mac)
				interfacePaths = append(interfacePaths, fmt.Sprintf("/etc/systemd/network/00-%s.network", iface))
			}
			cmdRunner.AddCmdResult("netmgr link_info --get", fakesys.FakeCmdResult{
				Stdout:     netmgrGetLinkInfo,
				Stderr:     "",
				ExitStatus: 0,
			})
			fs.SetGlob("/etc/systemd/network/*", interfacePaths)
		}

		stubInterfaces := func(physicalInterfaces map[string]boshsettings.Network) {
			stubInterfacesWithVirtual(physicalInterfaces, nil)
		}

		It("writes a network script for static and dynamic interfaces", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp":   dhcpNetwork,
				"ethstatic": staticNetwork,
			})

			cmdRunner.SetCmdCallback("netmgr ip4_address --set --interface ethstatic --mode static --addr 1.2.3.4/24 --gateway 3.4.5.6", func() {
				fs.WriteFileString("/etc/systemd/network/00-ethstatic.network", expectedNetworkConfigurationForStatic)
			})

			cmdRunner.SetCmdCallback("netmgr ip4_address --set --interface ethdhcp --mode dhcp", func() {
				fs.WriteFileString("/etc/systemd/network/00-ethdhcp.network", expectedNetworkConfigurationForDHCP)
			})

			err := netManager.SetupNetworking(boshsettings.Networks{"dhcp-network": dhcpNetwork, "static-network": staticNetwork}, nil)
			Expect(err).ToNot(HaveOccurred())

			staticConfig := fs.GetFileTestStat("/etc/systemd/network/00-ethstatic.network")
			Expect(staticConfig).ToNot(BeNil())
			Expect(staticConfig.StringContents()).To(Equal(expectedNetworkConfigurationForStatic))

			dhcpConfig := fs.GetFileTestStat("/etc/systemd/network/00-ethdhcp.network")
			Expect(dhcpConfig).ToNot(BeNil())
			Expect(dhcpConfig.StringContents()).To(Equal(expectedNetworkConfigurationForDHCP))
		})

		It("returns errors for lesser network devices", func() {
			err := netManager.SetupNetworking(boshsettings.Networks{"dhcp-network": dhcpNetwork, "static-network": staticNetwork}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("is greater than the number of network devices"))
		})

		It("returns errors when it can't create network interface configurations", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"ethstatic": staticNetwork,
			})

			cmdRunner.SetCmdCallback("netmgr ip4_address --set --interface ethstatic --mode static --addr 1.2.3.4/24 --gateway 3.4.5.6", func() {
				fs.WriteFileString("/etc/systemd/network/00-ethstatic.network", expectedNetworkConfigurationForStatic)
			})

			cmdRunner.SetCmdCallback("netmgr ip4_address --set --interface ethdhcp --mode dhcp", func() {
				fs.WriteFileString("/etc/systemd/network/00-ethdhcp.network", expectedNetworkConfigurationForDHCP)
			})
			staticNetwork.Netmask = "not an ip" //will cause InterfaceConfigurationCreator to fail
			err := netManager.SetupNetworking(boshsettings.Networks{"static-network": staticNetwork}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Creating interface configurations"))
		})

		Context("when manual networks were not configured with proper IP addresses", func() {
			BeforeEach(func() {
				interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
					boship.NewSimpleInterfaceAddress("ethstatic", "1.2.3.5"),
				}
			})

			It("fails", func() {
				stubInterfaces(map[string]boshsettings.Network{
					"ethstatic": staticNetwork,
				})

				cmdRunner.SetCmdCallback("netmgr ip4_address --set --interface ethstatic --mode static --addr 1.2.3.4/24 --gateway 3.4.5.6", func() {
					fs.WriteFileString("/etc/systemd/network/00-ethstatic.network", expectedNetworkConfigurationForStatic)
				})

				cmdRunner.SetCmdCallback("netmgr ip4_address --set --interface ethdhcp --mode dhcp", func() {
					fs.WriteFileString("/etc/systemd/network/00-ethdhcp.network", expectedNetworkConfigurationForDHCP)
				})
				errCh := make(chan error)
				err := netManager.SetupNetworking(boshsettings.Networks{"static-network": staticNetwork}, errCh)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Validating static network configuration"))
			})
		})

		It("broadcasts MAC addresses for all interfaces", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp":   dhcpNetwork,
				"ethstatic": staticNetwork,
			})

			cmdRunner.SetCmdCallback("netmgr ip4_address --set --interface ethstatic --mode static --addr 1.2.3.4/24 --gateway 3.4.5.6", func() {
				fs.WriteFileString("/etc/systemd/network/00-ethstatic.network", expectedNetworkConfigurationForStatic)
			})

			cmdRunner.SetCmdCallback("netmgr ip4_address --set --interface ethdhcp --mode dhcp", func() {
				fs.WriteFileString("/etc/systemd/network/00-ethdhcp.network", expectedNetworkConfigurationForDHCP)
			})
			errCh := make(chan error)
			err := netManager.SetupNetworking(boshsettings.Networks{"dhcp-network": dhcpNetwork, "static-network": staticNetwork}, errCh)
			Expect(err).ToNot(HaveOccurred())

			broadcastErr := <-errCh // wait for all arpings
			Expect(broadcastErr).ToNot(HaveOccurred())

			Expect(addressBroadcaster.BroadcastMACAddressesAddresses).To(Equal([]boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("ethstatic", "1.2.3.4"),
				boship.NewResolvingInterfaceAddress("ethdhcp", ipResolver),
			}))

		})

		It("skips vip networks", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp":   dhcpNetwork,
				"ethstatic": staticNetwork,
			})

			vipNetwork := boshsettings.Network{
				Type:    "vip",
				Default: []string{"dns"},
				DNS:     []string{"4.4.4.4", "5.5.5.5"},
				Mac:     "fake-vip-mac-address",
				IP:      "9.8.7.6",
			}

			cmdRunner.SetCmdCallback("netmgr ip4_address --set --interface ethstatic --mode static --addr 1.2.3.4/24 --gateway 3.4.5.6", func() {
				fs.WriteFileString("/etc/systemd/network/00-ethstatic.network", expectedNetworkConfigurationForStatic)
			})

			cmdRunner.SetCmdCallback("netmgr ip4_address --set --interface ethdhcp --mode dhcp", func() {
				fs.WriteFileString("/etc/systemd/network/00-ethdhcp.network", expectedNetworkConfigurationForDHCP)
			})
			err := netManager.SetupNetworking(boshsettings.Networks{
				"dhcp-network":   dhcpNetwork,
				"static-network": staticNetwork,
				"vip-network":    vipNetwork,
			}, nil)
			Expect(err).ToNot(HaveOccurred())

			networkConfig := fs.GetFileTestStat("/etc/systemd/network/00-ethstatic.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(expectedNetworkConfigurationForStatic))
		})

		Context("when no MAC address is provided in the settings", func() {
			var staticNetworkWithoutMAC boshsettings.Network

			BeforeEach(func() {
				staticNetworkWithoutMAC = boshsettings.Network{
					Type:    "manual",
					IP:      "1.2.3.4",
					Netmask: "255.255.255.0",
					Gateway: "3.4.5.6",
					DNS:     []string{"8.8.8.8", "9.9.9.9"},
					Default: []string{"dns"},
				}
			})

			It("configures network for single device", func() {
				stubInterfaces(
					map[string]boshsettings.Network{
						"ethstatic": staticNetwork,
					},
				)

				cmdRunner.SetCmdCallback("netmgr ip4_address --set --interface ethstatic --mode static --addr 1.2.3.4/24 --gateway 3.4.5.6", func() {
					fs.WriteFileString("/etc/systemd/network/00-ethstatic.network", expectedNetworkConfigurationForStatic)
				})

				err := netManager.SetupNetworking(boshsettings.Networks{
					"static-network": staticNetworkWithoutMAC,
				}, nil)
				Expect(err).ToNot(HaveOccurred())

				networkConfig := fs.GetFileTestStat("/etc/systemd/network/00-ethstatic.network")
				Expect(networkConfig).ToNot(BeNil())
				Expect(networkConfig.StringContents()).To(Equal(expectedNetworkConfigurationForStatic))
			})
		})
	})

}
