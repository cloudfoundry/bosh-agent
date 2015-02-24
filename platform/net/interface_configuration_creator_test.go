package net_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("InterfaceConfigurationCreator", func() {
	var (
		interfaceConfigurationCreator InterfaceConfigurationCreator
		staticNetwork                 boshsettings.Network
		dhcpNetwork                   boshsettings.Network
	)

	BeforeEach(func() {
		interfaceConfigurationCreator = NewInterfaceConfigurationCreator()
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
	})

	Describe("CreateInterfaceConfigurations", func() {
		It("creates interface configuratinos for each network and the matching interface", func() {
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
	})

	It("wraps errors calculating Network and Broadcast addresses", func() {
		invalidNetwork := boshsettings.Network{
			Type: "manual",
			IP:   "not an ip",
			Mac:  "invalid-network-mac-address",
		}
		interfacesByMAC := map[string]string{
			"invalid-network-mac-address": "static-interface-name",
		}

		_, _, err := interfaceConfigurationCreator.CreateInterfaceConfigurations(boshsettings.Networks{"foo": invalidNetwork}, interfacesByMAC)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Invalid ip or netmask"))
	})
})
