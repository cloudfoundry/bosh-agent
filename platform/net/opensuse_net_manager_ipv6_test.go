package net_test

import (
	"errors"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net"
	fakearp "github.com/cloudfoundry/bosh-agent/platform/net/arp/fakes"
	fakenet "github.com/cloudfoundry/bosh-agent/platform/net/fakes"
	boship "github.com/cloudfoundry/bosh-agent/platform/net/ip"
	fakeip "github.com/cloudfoundry/bosh-agent/platform/net/ip/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("OpensuseNetManager (IPv6)", func() {
	var (
		fs                            *fakesys.FakeFileSystem
		cmdRunner                     *fakesys.FakeCmdRunner
		ipResolver                    *fakeip.FakeResolver
		addressBroadcaster            *fakearp.FakeAddressBroadcaster
		interfaceAddrsProvider        *fakeip.FakeInterfaceAddressesProvider
		kernelIPv6                    *fakenet.FakeKernelIPv6
		netManager                    OpensuseNetManager
		interfaceConfigurationCreator InterfaceConfigurationCreator
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		cmdRunner = fakesys.NewFakeCmdRunner()
		ipResolver = &fakeip.FakeResolver{}
		logger := boshlog.NewLogger(boshlog.LevelNone)
		interfaceConfigurationCreator = NewInterfaceConfigurationCreator(logger)
		addressBroadcaster = &fakearp.FakeAddressBroadcaster{}
		interfaceAddrsProvider = &fakeip.FakeInterfaceAddressesProvider{}
		interfaceAddrsValidator := boship.NewInterfaceAddressesValidator(interfaceAddrsProvider)
		dnsValidator := NewDNSValidator(fs)
		kernelIPv6 = &fakenet.FakeKernelIPv6{}
		netManager = NewOpensuseNetManager(
			fs,
			cmdRunner,
			ipResolver,
			interfaceConfigurationCreator,
			interfaceAddrsValidator,
			dnsValidator,
			addressBroadcaster,
			kernelIPv6,
			logger,
		).(OpensuseNetManager)
	})

	writeNetworkDevice := func(iface string, macAddress string, isPhysical bool) string {
		interfacePath := fmt.Sprintf("/sys/class/net/%s", iface)
		fs.WriteFile(interfacePath, []byte{})
		if isPhysical {
			fs.WriteFile(fmt.Sprintf("/sys/class/net/%s/device", iface), []byte{})
		}
		fs.WriteFileString(fmt.Sprintf("/sys/class/net/%s/address", iface), fmt.Sprintf("%s\n", macAddress))

		return interfacePath
	}

	stubInterfacesWithVirtual := func(physicalInterfaces map[string]boshsettings.Network, virtualInterfaces []string) {
		interfacePaths := []string{}

		for iface, networkSettings := range physicalInterfaces {
			interfacePaths = append(interfacePaths, writeNetworkDevice(iface, networkSettings.Mac, true))
		}

		for _, iface := range virtualInterfaces {
			interfacePaths = append(interfacePaths, writeNetworkDevice(iface, "virtual", false))
		}

		fs.SetGlob("/sys/class/net/*", interfacePaths)
	}

	stubInterfaces := func(physicalInterfaces map[string]boshsettings.Network) {
		stubInterfacesWithVirtual(physicalInterfaces, nil)
	}

	scrubMultipleLines := func(in string) string {
		return strings.Replace(in, "\n\n\n", "\n\n", -1)
	}

	Describe("SetupNetworking", func() {
		BeforeEach(func() {
			err := fs.WriteFileString("/etc/resolv.conf", "nameserver 8.8.8.8\nnameserver 9.9.9.9")
			Expect(err).ToNot(HaveOccurred())

			err = fs.WriteFileString("/boot/grub/grub.conf", "")
			Expect(err).ToNot(HaveOccurred())

			interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("ethstatic1", "2601:646:100:e8e8::103"),
				boship.NewSimpleInterfaceAddress("ethstatic2", "1.2.3.4"),
				boship.NewSimpleInterfaceAddress("ethstatic3", "2601:646:100:eeee::10"),
			}
		})

		It("enables IPv6 if there are any IPv6 addresses", func() {
			static1Net := boshsettings.Network{
				Type:    "manual",
				IP:      "2601:646:100:e8e8::103",
				Netmask: "ffff:ffff:ffff:ffff:0000:0000:0000:0000",
				Gateway: "2601:646:100:e8e8::",
				Default: []string{"gateway", "dns"},
				DNS:     []string{"8.8.8.8", "9.9.9.9"},
				Mac:     "mac1",
			}

			stubInterfaces(map[string]boshsettings.Network{
				"ethstatic1": static1Net,
			})

			err := netManager.SetupNetworking(boshsettings.Networks{"net1": static1Net}, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(kernelIPv6.Enabled).To(BeTrue())
		})

		It("returns error if enabling IPv6 in kernel fails", func() {
			static1Net := boshsettings.Network{
				Type:    "manual",
				IP:      "2601:646:100:e8e8::103",
				Netmask: "ffff:ffff:ffff:ffff:0000:0000:0000:0000",
				Gateway: "2601:646:100:e8e8::",
				Default: []string{"gateway", "dns"},
				DNS:     []string{"8.8.8.8", "9.9.9.9"},
				Mac:     "mac1",
			}

			stubInterfaces(map[string]boshsettings.Network{
				"ethstatic1": static1Net,
			})

			kernelIPv6.EnableErr = errors.New("fake-err")

			err := netManager.SetupNetworking(boshsettings.Networks{"net1": static1Net}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-err"))
		})

		It("does not enable IPv6 if there aren't any IPv6 addresses", func() {
			static1Net := boshsettings.Network{
				Type:    "manual",
				IP:      "1.2.3.4",
				Default: nil,
				Netmask: "255.255.255.0",
				Gateway: "3.4.5.6",
				Mac:     "mac2",
			}

			stubInterfaces(map[string]boshsettings.Network{
				"ethstatic2": static1Net,
			})

			err := netManager.SetupNetworking(boshsettings.Networks{"net1": static1Net}, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(kernelIPv6.Enabled).To(BeFalse())
		})

		It("writes /etc/sysconfig/network/ifcfg-* with static inet6 configuration when manual network is used", func() {
			static1Net := boshsettings.Network{
				Type:    "manual",
				IP:      "2601:646:100:e8e8::103",
				Netmask: "ffff:ffff:ffff:ffff:0000:0000:0000:0000",
				Gateway: "2601:646:100:e8e8::",
				Default: []string{"gateway", "dns"},
				DNS:     []string{"8.8.8.8", "9.9.9.9"},
				Mac:     "mac1",
			}
			static2Net := boshsettings.Network{
				Type:    "manual",
				IP:      "1.2.3.4",
				Default: nil,
				Netmask: "255.255.255.0",
				Gateway: "3.4.5.6",
				Mac:     "mac2",
			}
			static3Net := boshsettings.Network{
				Type:    "manual",
				IP:      "2601:646:100:eeee::10",
				Netmask: "ffff:ffff:ffff:ffff:ffff:0000:0000:0000",
				Gateway: "2601:646:100:eeee::",
				Default: []string{},
				DNS:     []string{"8.8.8.8", "9.9.9.9"},
				Mac:     "mac3",
			}

			stubInterfaces(map[string]boshsettings.Network{
				"ethstatic1": static1Net,
				"ethstatic2": static2Net,
				"ethstatic3": static3Net,
			})

			err := netManager.SetupNetworking(boshsettings.Networks{
				"net1": static1Net,
				"net2": static2Net,
				"net3": static3Net,
			}, nil)
			Expect(err).ToNot(HaveOccurred())

			networkConfig := fs.GetFileTestStat("/etc/sysconfig/network/ifcfg-ethstatic1")
			Expect(networkConfig).ToNot(BeNil())
			Expect(scrubMultipleLines(networkConfig.StringContents())).To(Equal(`DEVICE=ethstatic1
BOOTPROTO=static
STARTMODE='auto'
IPADDR=2601:646:100:e8e8::103
NETMASK=ffff:ffff:ffff:ffff:0000:0000:0000:0000
BROADCAST=
GATEWAY=2601:646:100:e8e8::
`))

			networkConfig = fs.GetFileTestStat("/etc/sysconfig/network/ifcfg-ethstatic2")
			Expect(networkConfig).ToNot(BeNil())
			Expect(scrubMultipleLines(networkConfig.StringContents())).To(Equal(`DEVICE=ethstatic2
BOOTPROTO=static
STARTMODE='auto'
IPADDR=1.2.3.4
NETMASK=255.255.255.0
BROADCAST=1.2.3.255
GATEWAY=3.4.5.6
`))

			networkConfig = fs.GetFileTestStat("/etc/sysconfig/network/ifcfg-ethstatic3")
			Expect(networkConfig).ToNot(BeNil())
			Expect(scrubMultipleLines(networkConfig.StringContents())).To(Equal(`DEVICE=ethstatic3
BOOTPROTO=static
STARTMODE='auto'
IPADDR=2601:646:100:eeee::10
NETMASK=ffff:ffff:ffff:ffff:ffff:0000:0000:0000
BROADCAST=
GATEWAY=2601:646:100:eeee::
`))

		})
	})
})
