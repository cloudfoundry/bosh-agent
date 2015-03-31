package net_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("InterfaceConfigurationCreator", describeInterfaceConfigurationCreator)

func describeInterfaceConfigurationCreator() {
	var (
		interfaceConfigurationCreator InterfaceConfigurationCreator
		staticNetwork                 boshsettings.Network
		dhcpNetwork                   boshsettings.Network
	)

	BeforeEach(func() {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		interfaceConfigurationCreator = NewInterfaceConfigurationCreator(logger)
		dhcpNetwork = boshsettings.Network{
			Type:    "dynamic",
			Default: []string{"dns"},
			DNS:     []string{"8.8.8.8", "9.9.9.9"},
			Mac:     "fake-dhcp-mac-address",
		}
		staticNetwork = boshsettings.Network{
			IP:      "1.2.3.4",
			Netmask: "255.255.255.0",
			Gateway: "3.4.5.6",
			Mac:     "fake-static-mac-address",
		}
	})

	Describe("CreateInterfaceConfigurations", func() {
		Context("One network", func() {
			It("creates an interface configuration when matching interface exists", func() {
				networks := boshsettings.Networks{
					"foo": staticNetwork,
				}
				interfacesByMAC := map[string]string{
					"fake-static-mac-address": "static-interface-name",
				}

				staticInterfaceConfigurations, dhcpInterfaceConfigurations, err := interfaceConfigurationCreator.CreateInterfaceConfigurations(networks, interfacesByMAC)

				Expect(err).ToNot(HaveOccurred())

				Expect(staticInterfaceConfigurations).To(Equal([]StaticInterfaceConfiguration{
					StaticInterfaceConfiguration{
						Name:      "static-interface-name",
						Address:   "1.2.3.4",
						Netmask:   "255.255.255.0",
						Network:   "1.2.3.0",
						Broadcast: "1.2.3.255",
						Mac:       "fake-static-mac-address",
						Gateway:   "3.4.5.6",
					},
				}))

				Expect(len(dhcpInterfaceConfigurations)).To(Equal(0))
			})

			It("returns an error the network doesn't have a matching interface", func() {
				networks := boshsettings.Networks{
					"foo": staticNetwork,
				}
				interfacesByMAC := map[string]string{
					"fake-invalid-static-mac-address": "static-interface-name",
				}

				_, _, err := interfaceConfigurationCreator.CreateInterfaceConfigurations(networks, interfacesByMAC)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("No interface exists with MAC address 'fake-static-mac-address'"))
			})

			Context("With one interface", func() {
				It("creates an interface configuration even if no MAC address is specified for the network", func() {
					staticNetworkWithoutMAC := boshsettings.Network{
						Type:    "manual",
						IP:      "1.2.3.4",
						Netmask: "255.255.255.0",
						Gateway: "3.4.5.6",
					}
					networks := boshsettings.Networks{
						"foo": staticNetworkWithoutMAC,
					}
					interfacesByMAC := map[string]string{
						"fake-static-mac-address": "static-interface-name",
					}

					staticInterfaceConfigurations, dhcpInterfaceConfigurations, err := interfaceConfigurationCreator.CreateInterfaceConfigurations(networks, interfacesByMAC)

					Expect(err).ToNot(HaveOccurred())

					Expect(staticInterfaceConfigurations).To(Equal([]StaticInterfaceConfiguration{
						StaticInterfaceConfiguration{
							Name:      "static-interface-name",
							Address:   "1.2.3.4",
							Netmask:   "255.255.255.0",
							Network:   "1.2.3.0",
							Broadcast: "1.2.3.255",
							Mac:       "fake-static-mac-address",
							Gateway:   "3.4.5.6",
						},
					}))

					Expect(len(dhcpInterfaceConfigurations)).To(Equal(0))
				})
			})

			Context("With multiple interfaces", func() {
				It("returns an error if no MAC address is specified for the network", func() {
					staticNetworkWithoutMAC := boshsettings.Network{
						Type:    "manual",
						IP:      "1.2.3.4",
						Netmask: "255.255.255.0",
						Gateway: "3.4.5.6",
					}
					networks := boshsettings.Networks{
						"foo": staticNetworkWithoutMAC,
					}
					interfacesByMAC := map[string]string{
						"fake-static-mac-address":  "static-interface-name",
						"fake-static-mac-address2": "static-interface-name2",
					}

					_, _, err := interfaceConfigurationCreator.CreateInterfaceConfigurations(networks, interfacesByMAC)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Network 'foo' doesn't specify a MAC address"))

				})
			})

		})

		Context("Multiple networks", func() {
			It("creates interface configurations for each network when matching interfaces exist", func() {
				networks := boshsettings.Networks{
					"foo": staticNetwork,
					"bar": dhcpNetwork,
				}
				interfacesByMAC := map[string]string{
					"fake-dhcp-mac-address":   "dhcp-interface-name",
					"fake-static-mac-address": "static-interface-name",
				}

				staticInterfaceConfigurations, dhcpInterfaceConfigurations, err := interfaceConfigurationCreator.CreateInterfaceConfigurations(networks, interfacesByMAC)

				Expect(err).ToNot(HaveOccurred())

				Expect(staticInterfaceConfigurations).To(Equal([]StaticInterfaceConfiguration{
					StaticInterfaceConfiguration{
						Name:      "static-interface-name",
						Address:   "1.2.3.4",
						Netmask:   "255.255.255.0",
						Network:   "1.2.3.0",
						Broadcast: "1.2.3.255",
						Mac:       "fake-static-mac-address",
						Gateway:   "3.4.5.6",
					},
				}))

				Expect(dhcpInterfaceConfigurations).To(Equal([]DHCPInterfaceConfiguration{
					DHCPInterfaceConfiguration{
						Name: "dhcp-interface-name",
					},
				}))
			})

			It("returns an error if any network doesn't have a matching interface", func() {
				networks := boshsettings.Networks{
					"foo": staticNetwork,
					"bar": dhcpNetwork,
				}
				interfacesByMAC := map[string]string{
					"fake-dhcp-mac-address": "dhcp-interface-name",
				}

				_, _, err := interfaceConfigurationCreator.CreateInterfaceConfigurations(networks, interfacesByMAC)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("No interface exists with MAC address 'fake-static-mac-address'"))
			})

			It("returns an error if any network doesn't specify a MAC address", func() {
				staticNetworkWithoutMAC := boshsettings.Network{
					Type:    "manual",
					IP:      "1.2.3.4",
					Netmask: "255.255.255.0",
					Gateway: "3.4.5.6",
				}
				networks := boshsettings.Networks{
					"foo": staticNetworkWithoutMAC,
					"bar": dhcpNetwork,
				}
				interfacesByMAC := map[string]string{
					"fake-dhcp-mac-address": "dhcp-interface-name",
				}

				_, _, err := interfaceConfigurationCreator.CreateInterfaceConfigurations(networks, interfacesByMAC)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Network 'foo' doesn't specify a MAC address"))
			})
		})
	})

	It("wraps errors calculating Network and Broadcast addresses", func() {
		invalidNetwork := boshsettings.Network{
			Type:    "manual",
			IP:      "not an ip",
			Netmask: "not a valid mask",
			Mac:     "invalid-network-mac-address",
		}
		interfacesByMAC := map[string]string{
			"invalid-network-mac-address": "static-interface-name",
		}

		_, _, err := interfaceConfigurationCreator.CreateInterfaceConfigurations(boshsettings.Networks{"foo": invalidNetwork}, interfacesByMAC)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Invalid ip or netmask"))
	})
}
