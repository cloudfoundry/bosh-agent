// +build !windows

package net_test

import (
	"errors"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/factory"
	. "github.com/cloudfoundry/bosh-agent/platform/net"
	fakearp "github.com/cloudfoundry/bosh-agent/platform/net/arp/fakes"
	fakenet "github.com/cloudfoundry/bosh-agent/platform/net/fakes"
	boship "github.com/cloudfoundry/bosh-agent/platform/net/ip"
	fakeip "github.com/cloudfoundry/bosh-agent/platform/net/ip/fakes"
	"github.com/cloudfoundry/bosh-agent/platform/net/netfakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("ubuntuNetManager", func() {
	var (
		fs                            *fakesys.FakeFileSystem
		cmdRunner                     *fakesys.FakeCmdRunner
		ipResolver                    *fakeip.FakeResolver
		addressBroadcaster            *fakearp.FakeAddressBroadcaster
		interfaceAddrsProvider        *fakeip.FakeInterfaceAddressesProvider
		kernelIPv6                    *fakenet.FakeKernelIPv6
		netManager                    UbuntuNetManager
		interfaceConfigurationCreator InterfaceConfigurationCreator
		fakeMACAddressDetector        *netfakes.FakeMACAddressDetector
	)

	stubInterfaces := func(physicalInterfaces map[string]boshsettings.Network) {
		addresses := map[string]string{}
		for iface, networkSettings := range physicalInterfaces {
			addresses[networkSettings.Mac] = iface
		}

		fakeMACAddressDetector.DetectMacAddressesReturns(addresses, nil)
	}

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		cmdRunner = fakesys.NewFakeCmdRunner()
		ipResolver = &fakeip.FakeResolver{}
		logger := boshlog.NewLogger(boshlog.LevelNone)
		fakeMACAddressDetector = &netfakes.FakeMACAddressDetector{}
		interfaceConfigurationCreator = NewInterfaceConfigurationCreator(logger)
		addressBroadcaster = &fakearp.FakeAddressBroadcaster{}
		interfaceAddrsProvider = &fakeip.FakeInterfaceAddressesProvider{}
		interfaceAddrsValidator := boship.NewInterfaceAddressesValidator(interfaceAddrsProvider)
		dnsValidator := NewDNSValidator(fs)
		kernelIPv6 = &fakenet.FakeKernelIPv6{}
		netManager = NewUbuntuNetManager(
			fs,
			cmdRunner,
			ipResolver,
			fakeMACAddressDetector,
			interfaceConfigurationCreator,
			interfaceAddrsValidator,
			dnsValidator,
			addressBroadcaster,
			kernelIPv6,
			logger,
		).(UbuntuNetManager)
	})

	Describe("ComputeNetworkConfig", func() {
		Context("when there is one manual network and neither is marked as default for DNS", func() {
			It("should use the manual network for DNS", func() {
				networks := boshsettings.Networks{
					"manual": factory.Network{DNS: &[]string{"8.8.8.8"}}.Build(),
				}
				stubInterfaces(networks)
				_, _, dnsServers, err := netManager.ComputeNetworkConfig(networks)
				Expect(err).ToNot(HaveOccurred())
				Expect(dnsServers).To(Equal([]string{"8.8.8.8"}))
			})
		})

		Context("when there is a vip network and a manual network and neither is marked as default for DNS", func() {
			It("should use the manual network for DNS", func() {
				networks := boshsettings.Networks{
					"vip":    boshsettings.Network{Type: "vip"},
					"manual": factory.Network{Type: "manual", DNS: &[]string{"8.8.8.8"}}.Build(),
				}
				stubInterfaces(networks)
				_, _, dnsServers, err := netManager.ComputeNetworkConfig(networks)
				Expect(err).ToNot(HaveOccurred())
				Expect(dnsServers).To(Equal([]string{"8.8.8.8"}))
			})
		})
		Context("when there is a vip network and a manual network and the manual network is marked as default for DNS", func() {
			It("should use the manual network for DNS", func() {
				networks := boshsettings.Networks{
					"vip":    boshsettings.Network{Type: "vip"},
					"manual": factory.Network{Type: "manual", DNS: &[]string{"8.8.8.8"}, Default: []string{"dns"}}.Build(),
				}
				stubInterfaces(networks)
				_, _, dnsServers, err := netManager.ComputeNetworkConfig(networks)
				Expect(err).ToNot(HaveOccurred())
				Expect(dnsServers).To(Equal([]string{"8.8.8.8"}))
			})
		})

		Context("when specified more than one DNS", func() {
			It("extracts all DNS servers from the network configured as default DNS", func() {
				networks := boshsettings.Networks{
					"default": factory.Network{
						IP:      "10.10.0.32",
						Netmask: "255.255.255.0",
						Mac:     "aa::bb::cc",
						Default: []string{"dns", "gateway"},
						DNS:     &[]string{"54.209.78.6", "127.0.0.5"},
						Gateway: "10.10.0.1",
					}.Build(),
				}
				stubInterfaces(networks)
				staticInterfaceConfigurations, dhcpInterfaceConfigurations, dnsServers, err := netManager.ComputeNetworkConfig(networks)
				Expect(err).ToNot(HaveOccurred())

				Expect(staticInterfaceConfigurations).To(Equal([]StaticInterfaceConfiguration{
					{
						Name:                "default",
						Address:             "10.10.0.32",
						Netmask:             "255.255.255.0",
						Network:             "10.10.0.0",
						IsDefaultForGateway: true,
						Broadcast:           "10.10.0.255",
						Mac:                 "aa::bb::cc",
						Gateway:             "10.10.0.1",
					},
				}))
				Expect(dhcpInterfaceConfigurations).To(BeEmpty())
				Expect(dnsServers).To(Equal([]string{"54.209.78.6", "127.0.0.5"}))
			})
		})

		Context("when interface alias exists in network settings", func() {
			It("static interface configuration should be construted by alias name", func() {
				networks := boshsettings.Networks{
					"default": factory.Network{
						IP:      "10.10.0.32",
						Netmask: "255.255.255.0",
						Mac:     "aa::bb::cc",
						Default: []string{"dns", "gateway"},
						DNS:     &[]string{"54.209.78.6", "127.0.0.5"},
						Gateway: "10.10.0.1",
						Alias:   "eth0:0",
					}.Build(),
				}
				stubInterfaces(networks)
				staticInterfaceConfigurations, dhcpInterfaceConfigurations, dnsServers, err := netManager.ComputeNetworkConfig(networks)
				Expect(err).ToNot(HaveOccurred())

				Expect(staticInterfaceConfigurations).To(Equal([]StaticInterfaceConfiguration{
					{
						Name:                "default",
						Address:             "10.10.0.32",
						Netmask:             "255.255.255.0",
						Network:             "10.10.0.0",
						IsDefaultForGateway: true,
						Broadcast:           "10.10.0.255",
						Mac:                 "aa::bb::cc",
						Gateway:             "10.10.0.1",
					},
				}))
				Expect(dhcpInterfaceConfigurations).To(BeEmpty())
				Expect(dnsServers).To(Equal([]string{"54.209.78.6", "127.0.0.5"}))
			})
		})
	})

	Describe("SetupNetworking", func() {
		var (
			dhcpNetwork                                  boshsettings.Network
			staticNetwork                                boshsettings.Network
			expectedNetworkConfigurationForStaticAndDhcp string
			expectedResolvConfHead                       string
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
				Default: []string{"gateway"},
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
			expectedNetworkConfigurationForStaticAndDhcp = `# Generated by bosh-agent
auto lo
iface lo inet loopback

auto ethdhcp
iface ethdhcp inet dhcp

auto ethstatic
iface ethstatic inet static
    address 1.2.3.4
    network 1.2.3.0
    netmask 255.255.255.0
    broadcast 1.2.3.255
    gateway 3.4.5.6

dns-nameservers 8.8.8.8 9.9.9.9`
		})

		Context("networks is preconfigured", func() {
			var networks boshsettings.Networks
			BeforeEach(func() {
				dhcpNetwork.Preconfigured = true
				staticNetwork.Preconfigured = true
				networks = boshsettings.Networks{
					"first":  dhcpNetwork,
					"second": staticNetwork,
				}

				Expect(networks.IsPreconfigured()).To(BeTrue())
			})

			Context("when there are configured DNS servers", func() {
				BeforeEach(func() {
					networks = boshsettings.Networks{
						"first": dhcpNetwork,
					}
				})

				It("writes DNS to /etc/resolvconf/resolv.conf.d/base", func() {
					err := netManager.SetupNetworking(networks, nil)
					Expect(err).ToNot(HaveOccurred())

					resolvConfBase := fs.GetFileTestStat("/etc/resolvconf/resolv.conf.d/base")
					Expect(resolvConfBase).ToNot(BeNil())

					expectedResolvConfBase := `# Generated by bosh-agent
nameserver 8.8.8.8
nameserver 9.9.9.9
`
					Expect(resolvConfBase.StringContents()).To(Equal(expectedResolvConfBase))
				})

				Context("when writing to ../resolv.conf.d/base fails", func() {
					It("fails reporting the error", func() {
						fs.WriteFileError = errors.New("fake-write-file-error")

						err := netManager.SetupNetworking(networks, nil)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Writing to /etc/resolvconf/resolv.conf.d/base"))
					})
				})
			})

			Context("when no DNS servers are configured", func() {
				BeforeEach(func() {
					dhcpNetwork.DNS = []string{}
					networks = boshsettings.Networks{
						"first":  dhcpNetwork,
						"second": staticNetwork,
					}
				})

				Context("when could not read link /etc/resolv.conf", func() {
					It("fails reporting error", func() {
						fs.ReadAndFollowLinkError = errors.New("fake-read-link-error")

						err := netManager.SetupNetworking(networks, nil)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Reading /etc/resolv.conf symlink"))
					})
				})

				Context("when /etc/resolv.conf is no symlink", func() {
					BeforeEach(func() {
						err := fs.Symlink("/etc/resolv.conf", "/etc/resolv.conf")
						Expect(err).ToNot(HaveOccurred())

						err = fs.WriteFileString("/etc/resolv.conf", "fake-content")
						Expect(err).ToNot(HaveOccurred())
					})

					It("copies /etc/resolv.conf to .../resolv.conf.d/base", func() {
						err := netManager.SetupNetworking(networks, nil)
						Expect(err).ToNot(HaveOccurred())

						contents, err := fs.ReadFile("/etc/resolvconf/resolv.conf.d/base")
						Expect(err).ToNot(HaveOccurred())
						Expect(string(contents)).To(Equal("fake-content"))
					})

					Context("when copying fails", func() {
						It("fails reporting the error", func() {
							fs.CopyFileError = errors.New("fake-copy-error")

							err := netManager.SetupNetworking(networks, nil)
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("Copying /etc/resolv.conf for backwards compat"))
						})
					})
				})
			})

			It("forces /etc/resolv.conf to be a symlink", func() {
				err := netManager.SetupNetworking(networks, nil)
				Expect(err).ToNot(HaveOccurred())
				linkContents, err := fs.Readlink("/etc/resolv.conf")
				Expect(err).ToNot(HaveOccurred())

				expectedContents, err := filepath.Abs("/run/resolvconf/resolv.conf")
				Expect(err).ToNot(HaveOccurred())
				Expect(linkContents).To(Equal(expectedContents))
			})

			Context("when symlink command fails", func() {
				BeforeEach(func() {
					fs.SymlinkError = errors.New("fake-symlink-error")
				})

				It("fails reporting error", func() {
					err := netManager.SetupNetworking(networks, nil)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Setting up /etc/resolv.conf symlink"))
				})
			})

			It("writes dns servers in /etc/resolvconf/resolv.conf.d/base", func() {
				dhcpNetwork.Preconfigured = true
				staticNetwork.Preconfigured = true
				networks := boshsettings.Networks{
					"first":  dhcpNetwork,
					"second": staticNetwork,
				}

				Expect(networks.IsPreconfigured()).To(BeTrue())

				err := netManager.SetupNetworking(networks, nil)
				Expect(err).ToNot(HaveOccurred())

				resolvConfHead := fs.GetFileTestStat("/etc/resolvconf/resolv.conf.d/base")
				Expect(resolvConfHead).ToNot(BeNil())

				expectedResolvConfHead = `# Generated by bosh-agent
nameserver 8.8.8.8
nameserver 9.9.9.9
`
				Expect(resolvConfHead.StringContents()).To(Equal(expectedResolvConfHead))
			})

			It("run resolvconf -u to update resolv.conf", func() {
				dhcpNetwork.Preconfigured = true
				staticNetwork.Preconfigured = true
				networks := boshsettings.Networks{
					"first":  dhcpNetwork,
					"second": staticNetwork,
				}

				err := netManager.SetupNetworking(networks, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(cmdRunner.RunCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"resolvconf", "-u"}))
			})
		})

		It("configures gateway, broadcast and dns for default network only", func() {
			staticNetwork = boshsettings.Network{
				Type:    "manual",
				IP:      "1.2.3.4",
				Netmask: "255.255.255.0",
				Gateway: "3.4.5.6",
				Mac:     "fake-static-mac-address",
			}
			secondStaticNetwork := boshsettings.Network{
				Type:    "manual",
				IP:      "5.6.7.8",
				Netmask: "255.255.255.0",
				Gateway: "6.7.8.9",
				Mac:     "second-fake-static-mac-address",
				DNS:     []string{"8.8.8.8"},
				Default: []string{"gateway", "dns"},
			}

			stubInterfaces(map[string]boshsettings.Network{
				"eth0": staticNetwork,
				"eth1": secondStaticNetwork,
			})

			interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "1.2.3.4"),
				boship.NewSimpleInterfaceAddress("eth1", "5.6.7.8"),
			}

			err := netManager.SetupNetworking(boshsettings.Networks{
				"static-1": staticNetwork,
				"static-2": secondStaticNetwork,
			}, nil)
			Expect(err).ToNot(HaveOccurred())

			matches, err := fs.Ls("/etc/systemd/network/")
			Expect(err).NotTo(HaveOccurred())
			Expect(matches).To(ConsistOf(
				"/etc/systemd/network/10_eth0.network",
				"/etc/systemd/network/10_eth1.network",
			))
			networkConfig := fs.GetFileTestStat("/etc/systemd/network/10_eth0.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=eth0

[Address]
Address=1.2.3.4/24


[Network]


DNS=8.8.8.8


[Route]
`))
			networkConfig = fs.GetFileTestStat("/etc/systemd/network/10_eth1.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=eth1

[Address]
Address=5.6.7.8/24
Broadcast=5.6.7.255

[Network]
Gateway=6.7.8.9

DNS=8.8.8.8


[Route]
`))
		})

		It("ensures the only interfaces configured are the ones currently configured when SetupNetworking is re-run", func() {
			By("Pre-configuring the network to have two devices", func() {
				staticNetwork = boshsettings.Network{
					Type:    "manual",
					IP:      "1.2.3.4",
					Netmask: "255.255.255.0",
					Gateway: "3.4.5.6",
					Mac:     "fake-static-mac-address",
				}
				secondStaticNetwork := boshsettings.Network{
					Type:    "manual",
					IP:      "5.6.7.8",
					Netmask: "255.255.255.0",
					Gateway: "6.7.8.9",
					Mac:     "second-fake-static-mac-address",
					DNS:     []string{"8.8.8.8"},
					Default: []string{"gateway", "dns"},
				}

				stubInterfaces(map[string]boshsettings.Network{
					"eth0": staticNetwork,
					"eth1": secondStaticNetwork,
				})

				interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
					boship.NewSimpleInterfaceAddress("eth0", "1.2.3.4"),
					boship.NewSimpleInterfaceAddress("eth1", "5.6.7.8"),
				}

				err := netManager.SetupNetworking(boshsettings.Networks{
					"static-1": staticNetwork,
					"static-2": secondStaticNetwork,
				}, nil)
				Expect(err).ToNot(HaveOccurred())

				matches, err := fs.Ls("/etc/systemd/network/")
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(ConsistOf(
					"/etc/systemd/network/10_eth0.network",
					"/etc/systemd/network/10_eth1.network",
				))
				networkConfig := fs.GetFileTestStat("/etc/systemd/network/10_eth0.network")
				Expect(networkConfig).ToNot(BeNil())
				networkConfig = fs.GetFileTestStat("/etc/systemd/network/10_eth1.network")
				Expect(networkConfig).ToNot(BeNil())
			})

			By("Reconfiguring to only have one", func() {
				// Otherwise, running SetupNetworking twice with different networks could leak networks
				staticNetwork = boshsettings.Network{
					Type:    "manual",
					IP:      "5.6.7.8",
					Netmask: "255.255.255.0",
					Gateway: "6.7.8.9",
					Mac:     "second-fake-static-mac-address",
					DNS:     []string{"8.8.8.8"},
					Default: []string{"gateway", "dns"},
				}

				stubInterfaces(map[string]boshsettings.Network{
					"eth0": staticNetwork,
				})

				interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
					boship.NewSimpleInterfaceAddress("eth0", "5.6.7.8"),
				}

				err := netManager.SetupNetworking(boshsettings.Networks{
					"static-1": staticNetwork,
				}, nil)
				Expect(err).ToNot(HaveOccurred())

				matches, err := fs.Ls("/etc/systemd/network/")
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(ConsistOf(
					"/etc/systemd/network/10_eth0.network",
				))
				networkConfig := fs.GetFileTestStat("/etc/systemd/network/10_eth0.network")
				Expect(networkConfig).ToNot(BeNil())
				Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=eth0

[Address]
Address=5.6.7.8/24
Broadcast=5.6.7.255

[Network]
Gateway=6.7.8.9

DNS=8.8.8.8


[Route]
`))
			})
		})

		It("configures postup routes for static network", func() {
			staticNetwork = boshsettings.Network{
				Type:    "manual",
				IP:      "1.2.3.4",
				Netmask: "255.255.255.0",
				Gateway: "3.4.5.6",
				Mac:     "fake-static-mac-address",
				Routes: []boshsettings.Route{
					boshsettings.Route{
						Destination: "10.0.0.0",
						Gateway:     "3.4.5.6",
						Netmask:     "255.0.0.0",
					},
				},
			}
			secondStaticNetwork := boshsettings.Network{
				Type:    "manual",
				IP:      "5.6.7.8",
				Netmask: "255.255.255.0",
				Gateway: "6.7.8.9",
				Mac:     "second-fake-static-mac-address",
				DNS:     []string{"8.8.8.8"},
				Default: []string{"gateway", "dns"},
			}

			stubInterfaces(map[string]boshsettings.Network{
				"eth0": staticNetwork,
				"eth1": secondStaticNetwork,
			})

			interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "1.2.3.4"),
				boship.NewSimpleInterfaceAddress("eth1", "5.6.7.8"),
			}

			err := netManager.SetupNetworking(boshsettings.Networks{
				"static-1": staticNetwork,
				"static-2": secondStaticNetwork,
			}, nil)
			Expect(err).ToNot(HaveOccurred())

			matches, err := fs.Ls("/etc/systemd/network/")
			Expect(err).ToNot(HaveOccurred())
			Expect(matches).To(ConsistOf(
				"/etc/systemd/network/10_eth0.network",
				"/etc/systemd/network/10_eth1.network",
			))
			networkConfig := fs.GetFileTestStat("/etc/systemd/network/10_eth0.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=eth0

[Address]
Address=1.2.3.4/24


[Network]


DNS=8.8.8.8


[Route]

Destination=10.0.0.0/8
Gateway=3.4.5.6
`))
			networkConfig = fs.GetFileTestStat("/etc/systemd/network/10_eth1.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=eth1

[Address]
Address=5.6.7.8/24
Broadcast=5.6.7.255

[Network]
Gateway=6.7.8.9

DNS=8.8.8.8


[Route]
`))
		})

		It("writes /etc/network/interfaces without dns-namservers if there are no dns servers", func() {
			staticNetworkWithoutDNS := boshsettings.Network{
				Type:    "manual",
				IP:      "1.2.3.4",
				Default: []string{"gateway"},
				Netmask: "255.255.255.0",
				Gateway: "3.4.5.6",
				Mac:     "fake-static-mac-address",
			}

			stubInterfaces(map[string]boshsettings.Network{
				"ethstatic": staticNetworkWithoutDNS,
			})

			err := netManager.SetupNetworking(boshsettings.Networks{"static-network": staticNetworkWithoutDNS}, nil)
			Expect(err).ToNot(HaveOccurred())

			matches, err := fs.Ls("/etc/systemd/network/")
			Expect(err).ToNot(HaveOccurred())
			Expect(matches).To(ConsistOf("/etc/systemd/network/10_ethstatic.network"))

			networkConfig := fs.GetFileTestStat("/etc/systemd/network/10_ethstatic.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=ethstatic

[Address]
Address=1.2.3.4/24
Broadcast=1.2.3.255

[Network]
Gateway=3.4.5.6



[Route]
`))
		})

		It("returns errors from writing the network configuration", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"dhcp":   dhcpNetwork,
				"static": staticNetwork,
			})
			fs.WriteFileError = errors.New("fs-write-file-error")
			err := netManager.SetupNetworking(boshsettings.Networks{"dhcp-network": dhcpNetwork, "static-network": staticNetwork}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fs-write-file-error"))
		})

		It("returns errors when it can't creating network interface configurations", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp":   dhcpNetwork,
				"ethstatic": staticNetwork,
			})
			staticNetwork.Netmask = "not an ip" //will cause InterfaceConfigurationCreator to fail
			err := netManager.SetupNetworking(boshsettings.Networks{"static-network": staticNetwork}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Creating interface configurations"))
		})

		It("returns errors when there a netmask cannot be converted to a CIDR", func() {
			staticNetwork = boshsettings.Network{
				Type:    "manual",
				IP:      "1.2.3.4",
				Netmask: "255.0.255.0",
				Gateway: "3.4.5.6",
				Mac:     "fake-static-mac-address",
			}

			stubInterfaces(map[string]boshsettings.Network{
				"eth0": staticNetwork,
			})

			interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "1.2.3.4"),
			}

			err := netManager.SetupNetworking(boshsettings.Networks{
				"static-1": staticNetwork,
			}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(`Updating network configs: Writing network configuration: Updating network configuration for eth0: Generating config from template eth0: template: eth0:6:58: executing "eth0" at <.InterfaceConfig.CIDR>: error calling CIDR: netmask cannot be converted to CIDR: 255.0.255.0`))
		})

		It("writes a dhcp configuration if there are dhcp networks", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp":   dhcpNetwork,
				"ethstatic": staticNetwork,
			})

			err := netManager.SetupNetworking(boshsettings.Networks{"dhcp-network": dhcpNetwork, "static-network": staticNetwork}, nil)
			Expect(err).ToNot(HaveOccurred())

			dhcpConfig := fs.GetFileTestStat("/etc/dhcp/dhclient.conf")
			Expect(dhcpConfig).ToNot(BeNil())
			Expect(dhcpConfig.StringContents()).To(Equal(`# Generated by bosh-agent

option rfc3442-classless-static-routes code 121 = array of unsigned integer 8;

send host-name = gethostname();

request subnet-mask, broadcast-address, time-offset, routers,
	domain-name, domain-name-servers, domain-search, host-name,
	netbios-name-servers, netbios-scope, interface-mtu,
	rfc3442-classless-static-routes, ntp-servers;

prepend domain-name-servers 8.8.8.8, 9.9.9.9;
`))

		})

		It("writes a dhcp configuration without prepended dns servers if there are no dns servers specified", func() {
			dhcpNetworkWithoutDNS := boshsettings.Network{
				Type: "dynamic",
				Mac:  "fake-dhcp-mac-address",
			}

			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp": dhcpNetwork,
			})

			err := netManager.SetupNetworking(boshsettings.Networks{"dhcp-network": dhcpNetworkWithoutDNS}, nil)
			Expect(err).ToNot(HaveOccurred())

			dhcpConfig := fs.GetFileTestStat("/etc/dhcp/dhclient.conf")
			Expect(dhcpConfig).ToNot(BeNil())
			Expect(dhcpConfig.StringContents()).To(Equal(`# Generated by bosh-agent

option rfc3442-classless-static-routes code 121 = array of unsigned integer 8;

send host-name = gethostname();

request subnet-mask, broadcast-address, time-offset, routers,
	domain-name, domain-name-servers, domain-search, host-name,
	netbios-name-servers, netbios-scope, interface-mtu,
	rfc3442-classless-static-routes, ntp-servers;

`))

		})

		It("returns an error if it can't write a dhcp configuration", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp":   dhcpNetwork,
				"ethstatic": staticNetwork,
			})

			fs.WriteFileErrors["/etc/dhcp/dhclient.conf"] = errors.New("dhclient.conf-write-error")

			err := netManager.SetupNetworking(boshsettings.Networks{"dhcp-network": dhcpNetwork, "static-network": staticNetwork}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("dhclient.conf-write-error"))
		})

		It("doesn't write a dhcp configuration if there are no dhcp networks", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"ethstatic": staticNetwork,
			})

			err := netManager.SetupNetworking(boshsettings.Networks{"static-network": staticNetwork}, nil)
			Expect(err).ToNot(HaveOccurred())

			dhcpConfig := fs.GetFileTestStat("/etc/dhcp/dhclient.conf")
			Expect(dhcpConfig).To(BeNil())
		})

		It("restarts the networks if /etc/network/interfaces changes", func() {
			initialDhcpConfig := `# Generated by bosh-agent

option rfc3442-classless-static-routes code 121 = array of unsigned integer 8;

send host-name = gethostname();

request subnet-mask, broadcast-address, time-offset, routers,
	domain-name, domain-name-servers, domain-search, host-name,
	netbios-name-servers, netbios-scope, interface-mtu,
	rfc3442-classless-static-routes, ntp-servers;

prepend domain-name-servers 8.8.8.8, 9.9.9.9;
`

			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp":   dhcpNetwork,
				"ethstatic": staticNetwork,
			})

			fs.WriteFileString("/etc/dhcp/dhclient.conf", initialDhcpConfig)

			// check that config files change after stop and before start
			cmdRunner.SetCmdCallback("ip --force link set ethdhcp down", func() {
				Expect(fs.ReadFileString("/etc/dhcp/dhclient.conf")).To(Equal(initialDhcpConfig))
			})

			cmdRunner.SetCmdCallback("ip --force link set ethstatic down", func() {
				Expect(fs.ReadFileString("/etc/dhcp/dhclient.conf")).To(Equal(initialDhcpConfig))
			})

			cmdRunner.SetCmdCallback("ip --force link up ethdhcp", func() {
				Expect(fs.ReadFileString("/etc/dhcp/dhclient.conf")).To(Equal(initialDhcpConfig))
			})
			cmdRunner.SetCmdCallback("ip --force link up ethstatic", func() {
				Expect(fs.ReadFileString("/etc/dhcp/dhclient.conf")).To(Equal(initialDhcpConfig))
			})

			err := netManager.SetupNetworking(boshsettings.Networks{"dhcp-network": dhcpNetwork, "static-network": staticNetwork}, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(cmdRunner.RunCommands)).To(Equal(5))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"pkill", "dhclient"}))
			Expect(cmdRunner.RunCommands[1:3]).To(ContainElement([]string{"resolvconf", "-d", "ethdhcp.dhclient"}))
			Expect(cmdRunner.RunCommands[1:3]).To(ContainElement([]string{"resolvconf", "-d", "ethstatic.dhclient"}))
			Expect(cmdRunner.RunCommands[3]).To(Equal([]string{"/var/vcap/bosh/bin/restart_networking"}))
			Expect(cmdRunner.RunCommands[4]).To(Equal([]string{"resolvconf", "-u"}))

			Expect(fs.ReadFileString("/etc/dhcp/dhclient.conf")).To(Equal(initialDhcpConfig))
		})

		It("doesn't restart the networks if /etc/network/interfaces and /etc/dhcp/dhclient.conf don't change", func() {
			initialDhcpConfig := `# Generated by bosh-agent

option rfc3442-classless-static-routes code 121 = array of unsigned integer 8;

send host-name = gethostname();

request subnet-mask, broadcast-address, time-offset, routers,
	domain-name, domain-name-servers, domain-search, host-name,
	netbios-name-servers, netbios-scope, interface-mtu,
	rfc3442-classless-static-routes, ntp-servers;

prepend domain-name-servers 8.8.8.8, 9.9.9.9;
`
			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp":   dhcpNetwork,
				"ethstatic": staticNetwork,
			})

			fs.WriteFileString("/etc/network/interfaces", expectedNetworkConfigurationForStaticAndDhcp)
			fs.WriteFileString("/etc/dhcp/dhclient.conf", initialDhcpConfig)

			err := netManager.SetupNetworking(boshsettings.Networks{"dhcp-network": dhcpNetwork, "static-network": staticNetwork}, nil)
			Expect(err).ToNot(HaveOccurred())

			networkConfig := fs.GetFileTestStat("/etc/network/interfaces")
			Expect(networkConfig.StringContents()).To(Equal(expectedNetworkConfigurationForStaticAndDhcp))
			dhcpConfig := fs.GetFileTestStat("/etc/dhcp/dhclient.conf")
			Expect(dhcpConfig.StringContents()).To(Equal(initialDhcpConfig))

			Expect(len(cmdRunner.RunCommands)).To(Equal(5))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"pkill", "dhclient"}))
			Expect(cmdRunner.RunCommands[1:3]).To(ContainElement([]string{"resolvconf", "-d", "ethdhcp.dhclient"}))
			Expect(cmdRunner.RunCommands[1:3]).To(ContainElement([]string{"resolvconf", "-d", "ethstatic.dhclient"}))
			Expect(cmdRunner.RunCommands[3]).To(Equal([]string{"/var/vcap/bosh/bin/restart_networking"}))
			Expect(cmdRunner.RunCommands[4]).To(Equal([]string{"resolvconf", "-u"}))
		})

		It("restarts the networks if /etc/dhcp/dhclient.conf changes", func() {
			initialDhcpConfig := "initial-dhcp-config"

			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp":   dhcpNetwork,
				"ethstatic": staticNetwork,
			})

			fs.WriteFileString("/etc/dhcp/dhclient.conf", initialDhcpConfig)

			// check that config files change after stop and before start
			cmdRunner.SetCmdCallback("ip --force link set ethdhcp down", func() {
				Expect(fs.ReadFileString("/etc/dhcp/dhclient.conf")).To(Equal(initialDhcpConfig))
			})

			cmdRunner.SetCmdCallback("ip --force link set ethstatic down", func() {
				Expect(fs.ReadFileString("/etc/dhcp/dhclient.conf")).To(Equal(initialDhcpConfig))
			})

			cmdRunner.SetCmdCallback("ip --force link up ethdhcp", func() {
				Expect(fs.ReadFileString("/etc/dhcp/dhclient.conf")).To(Equal(initialDhcpConfig))
			})
			cmdRunner.SetCmdCallback("ip --force link up ethstatic", func() {
				Expect(fs.ReadFileString("/etc/dhcp/dhclient.conf")).To(Equal(initialDhcpConfig))
			})

			err := netManager.SetupNetworking(boshsettings.Networks{"dhcp-network": dhcpNetwork, "static-network": staticNetwork}, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(cmdRunner.RunCommands)).To(Equal(5))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"pkill", "dhclient"}))
			Expect(cmdRunner.RunCommands[1:3]).To(ContainElement([]string{"resolvconf", "-d", "ethdhcp.dhclient"}))
			Expect(cmdRunner.RunCommands[1:3]).To(ContainElement([]string{"resolvconf", "-d", "ethstatic.dhclient"}))
			Expect(cmdRunner.RunCommands[3]).To(Equal([]string{"/var/vcap/bosh/bin/restart_networking"}))
			Expect(cmdRunner.RunCommands[4]).To(Equal([]string{"resolvconf", "-u"}))

			Expect(fs.ReadFileString("/etc/dhcp/dhclient.conf")).ToNot(Equal(initialDhcpConfig))
		})

		It("broadcasts MAC addresses for all interfaces", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp":   dhcpNetwork,
				"ethstatic": staticNetwork,
			})

			err := netManager.SetupNetworking(boshsettings.Networks{"dhcp-network": dhcpNetwork, "static-network": staticNetwork}, nil)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []boship.InterfaceAddress { return addressBroadcaster.Value() }).Should(
				Equal([]boship.InterfaceAddress{
					boship.NewSimpleInterfaceAddress("ethstatic", "1.2.3.4"),
					boship.NewResolvingInterfaceAddress("ethdhcp", ipResolver),
				}),
			)
		})

		It("skips vip networks", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"ethdhcp":   dhcpNetwork,
				"ethstatic": staticNetwork,
			})

			vipNetwork := boshsettings.Network{
				Type:    "vip",
				Default: []string{"dns"},
				DNS:     []string{"8.8.8.8", "9.9.9.9"},
				Mac:     "fake-vip-mac-address",
				IP:      "9.8.7.6",
			}

			err := netManager.SetupNetworking(boshsettings.Networks{
				"dhcp-network":   dhcpNetwork,
				"static-network": staticNetwork,
				"vip-network":    vipNetwork,
			}, nil)
			Expect(err).ToNot(HaveOccurred())

			matches, err := fs.Ls("/etc/systemd/network/")
			Expect(err).NotTo(HaveOccurred())
			Expect(matches).To(ConsistOf(
				"/etc/systemd/network/10_ethdhcp.network",
				"/etc/systemd/network/10_ethstatic.network",
			))

			networkConfig := fs.GetFileTestStat("/etc/systemd/network/10_ethdhcp.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=ethdhcp

[Network]
DHCP=yes

DNS=8.8.8.8
DNS=9.9.9.9


[Route]
`))

			networkConfig = fs.GetFileTestStat("/etc/systemd/network/10_ethstatic.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=ethstatic

[Address]
Address=1.2.3.4/24
Broadcast=1.2.3.255

[Network]
Gateway=3.4.5.6

DNS=8.8.8.8
DNS=9.9.9.9


[Route]
`))
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

				errCh := make(chan error)
				err := netManager.SetupNetworking(boshsettings.Networks{"static-network": staticNetwork}, errCh)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Validating static network configuration"))
			})
		})

		Context("when dns is not properly configured", func() {
			BeforeEach(func() {
				fs.WriteFileString("/etc/resolv.conf", "")
			})

			It("fails", func() {
				staticNetwork = boshsettings.Network{
					Type:    "manual",
					IP:      "1.2.3.4",
					Default: []string{"dns"},
					DNS:     []string{"8.8.8.8"},
					Netmask: "255.255.255.0",
					Gateway: "3.4.5.6",
					Mac:     "fake-static-mac-address",
				}

				stubInterfaces(map[string]boshsettings.Network{
					"ethstatic": staticNetwork,
				})

				errCh := make(chan error)
				err := netManager.SetupNetworking(boshsettings.Networks{"static-network": staticNetwork}, errCh)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Validating dns configuration"))
			})
		})

		Context("when no MAC address is provided in the settings", func() {
			It("configures network for single device", func() {
				staticNetworkWithoutMAC := boshsettings.Network{
					Type:    "manual",
					IP:      "2.2.2.2",
					Default: []string{"gateway"},
					Netmask: "255.255.255.0",
					Gateway: "3.4.5.6",
				}

				stubInterfaces(
					map[string]boshsettings.Network{
						"ethstatic": staticNetwork,
					},
				)
				interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
					boship.NewSimpleInterfaceAddress("ethstatic", "2.2.2.2"),
				}

				err := netManager.SetupNetworking(boshsettings.Networks{
					"static-network": staticNetworkWithoutMAC,
				}, nil)
				Expect(err).ToNot(HaveOccurred())

				matches, err := fs.Ls("/etc/systemd/network/")
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(ConsistOf(
					"/etc/systemd/network/10_ethstatic.network",
				))

				networkConfig := fs.GetFileTestStat("/etc/systemd/network/10_ethstatic.network")
				Expect(networkConfig).ToNot(BeNil())
				Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=ethstatic

[Address]
Address=2.2.2.2/24
Broadcast=2.2.2.255

[Network]
Gateway=3.4.5.6



[Route]
`))
			})
		})

		Context("when manual networks were configured with portable IP", func() {
			var (
				portableNetwork boshsettings.Network
				staticNetwork   boshsettings.Network
				staticNetwork1  boshsettings.Network
			)
			BeforeEach(func() {
				portableNetwork = boshsettings.Network{
					Type:     "manual",
					IP:       "10.112.166.136",
					Netmask:  "255.255.255.192",
					Resolved: false,
					UseDHCP:  false,
					DNS:      []string{"8.8.8.8"},
					Alias:    "eth0:0",
				}
				staticNetwork = boshsettings.Network{
					Type:     "dynamic",
					IP:       "169.50.68.75",
					Netmask:  "255.255.255.224",
					Gateway:  "169.50.68.65",
					Default:  []string{"gateway", "dns"},
					Resolved: false,
					UseDHCP:  false,
					DNS:      []string{"8.8.8.8", "10.0.80.11", "10.0.80.12"},
					Mac:      "06:64:d4:7d:63:71",
					Alias:    "eth1",
				}
				staticNetwork1 = boshsettings.Network{
					Type:     "dynamic",
					IP:       "10.112.39.113",
					Netmask:  "255.255.255.128",
					Resolved: false,
					UseDHCP:  false,
					DNS:      []string{"8.8.8.8", "10.0.80.11", "10.0.80.12"},
					Mac:      "06:b7:e8:0c:38:d8",
					Alias:    "eth0",
				}
				interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
					boship.NewSimpleInterfaceAddress("eth0", "10.112.39.113"),
					boship.NewSimpleInterfaceAddress("eth1", "169.50.68.75"),
				}
				fs.WriteFileString("/etc/resolv.conf", `
nameserver 8.8.8.8
nameserver 10.0.80.11
nameserver 10.0.80.12
`)
			})

			scrubMultipleLines := func(in string) string {
				return strings.Replace(in, "\n\n\n", "\n\n", -1)
			}

			It("succeeds", func() {
				stubInterfaces(map[string]boshsettings.Network{
					"eth1": staticNetwork,
					"eth0": staticNetwork1,
				})

				err := netManager.SetupNetworking(boshsettings.Networks{"default": portableNetwork, "dynamic": staticNetwork, "dynamic_1": staticNetwork1}, nil)

				Eventually(func() []boship.InterfaceAddress { return addressBroadcaster.Value() }).Should(
					Equal([]boship.InterfaceAddress{
						boship.NewSimpleInterfaceAddress("eth0", "10.112.39.113"),
						boship.NewSimpleInterfaceAddress("eth1", "169.50.68.75"),
					}),
				)

				matches, err := fs.Ls("/etc/systemd/network/")
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(ConsistOf(
					"/etc/systemd/network/10_eth0.network",
					"/etc/systemd/network/10_eth0:0.network",
					"/etc/systemd/network/10_eth1.network",
				))

				networkConfig := fs.GetFileTestStat("/etc/systemd/network/10_eth0.network")
				Expect(networkConfig).ToNot(BeNil())
				Expect(scrubMultipleLines(networkConfig.StringContents())).To(Equal(`# Generated by bosh-agent
[Match]
Name=eth0

[Address]
Address=10.112.39.113/25

[Network]

DNS=8.8.8.8
DNS=10.0.80.11
DNS=10.0.80.12

[Route]
`))
				networkConfig = fs.GetFileTestStat("/etc/systemd/network/10_eth0:0.network")
				Expect(networkConfig).ToNot(BeNil())
				Expect(scrubMultipleLines(networkConfig.StringContents())).To(Equal(`# Generated by bosh-agent
[Match]
Name=eth0:0

[Address]
Address=10.112.166.136/26

[Network]

DNS=8.8.8.8
DNS=10.0.80.11
DNS=10.0.80.12

[Route]
`))
				networkConfig = fs.GetFileTestStat("/etc/systemd/network/10_eth1.network")
				Expect(networkConfig).ToNot(BeNil())
				Expect(scrubMultipleLines(networkConfig.StringContents())).To(Equal(`# Generated by bosh-agent
[Match]
Name=eth1

[Address]
Address=169.50.68.75/27
Broadcast=169.50.68.95

[Network]
Gateway=169.50.68.65

DNS=8.8.8.8
DNS=10.0.80.11
DNS=10.0.80.12

[Route]
`))
			})
		})
	})

	Describe("GetConfiguredNetworkInterfaces", func() {
		Context("when there are network devices", func() {
			BeforeEach(func() {
				stubInterfaces(map[string]boshsettings.Network{
					"fake-eth0": boshsettings.Network{Mac: "aa:bb"},
					"fake-eth1": boshsettings.Network{Mac: "cc:dd"},
					"fake-eth2": boshsettings.Network{Mac: "ee:ff"},
					"fake-eth3": boshsettings.Network{Mac: "yy:zz"},
				})
			})

			It("returns networks that are defined in /etc/network/interfaces", func() {
				fs.WriteFileString("/etc/systemd/network/10_fake-eth0.network", ``)
				fs.WriteFileString("/etc/systemd/network/10_fake-eth2.network", ``)

				interfaces, err := netManager.GetConfiguredNetworkInterfaces()
				Expect(err).ToNot(HaveOccurred())

				Expect(interfaces).To(ConsistOf("fake-eth0", "fake-eth2"))
			})
		})

		Context("when there are no network devices", func() {
			It("returns empty list", func() {
				interfaces, err := netManager.GetConfiguredNetworkInterfaces()
				Expect(err).ToNot(HaveOccurred())
				Expect(interfaces).To(Equal([]string{}))
			})
		})
	})

	Describe("SetupIPv6", func() {
		var (
			config boshsettings.IPv6
			stopCh chan struct{}
		)

		BeforeEach(func() {
			config = boshsettings.IPv6{}
			stopCh = make(chan struct{}, 1)
		})

		act := func() error { return netManager.SetupIPv6(config, stopCh) }

		Context("when IPv6 is enabled by the user", func() {
			It("enables IPv6", func() {
				config.Enable = true
				Expect(act()).ToNot(HaveOccurred())
				Expect(kernelIPv6.Enabled).To(BeTrue())
			})
		})

		Context("when IPv6 is NOT enabled by the user", func() {
			It("does not enable IPv6", func() {
				config.Enable = false
				Expect(act()).ToNot(HaveOccurred())
				Expect(kernelIPv6.Enabled).To(BeFalse())
			})
		})
	})
})
