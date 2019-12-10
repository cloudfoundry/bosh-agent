// +build !windows

package net_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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

var _ = Describe("UbuntuNetManager (IPv6)", func() {
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

	Describe("SetupNetworking", func() {
		BeforeEach(func() {
			err := fs.WriteFileString("/etc/resolv.conf", "nameserver 8.8.8.8\nnameserver 9.9.9.9")
			Expect(err).ToNot(HaveOccurred())

			err = fs.WriteFileString("/boot/grub/grub.cfg", "")
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

		It("writes /etc/network/interfaces with static inet6 configuration when manual network is used", func() {
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

			matches, err := fs.Ls("/etc/systemd/network/")
			Expect(err).NotTo(HaveOccurred())
			Expect(matches).To(ConsistOf(
				"/etc/systemd/network/10_ethstatic1.network",
				"/etc/systemd/network/10_ethstatic2.network",
				"/etc/systemd/network/10_ethstatic3.network",
			))
			networkConfig := fs.GetFileTestStat("/etc/systemd/network/10_ethstatic1.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=ethstatic1

[Address]
Address=2601:646:100:e8e8::103/64

[Network]
Gateway=2601:646:100:e8e8::
IPv6AcceptRA=true
DNS=8.8.8.8
DNS=9.9.9.9

`))
			networkConfig = fs.GetFileTestStat("/etc/systemd/network/10_ethstatic2.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=ethstatic2

[Address]
Address=1.2.3.4/24

[Network]
DNS=8.8.8.8
DNS=9.9.9.9

`))
			networkConfig = fs.GetFileTestStat("/etc/systemd/network/10_ethstatic3.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=ethstatic3

[Address]
Address=2601:646:100:eeee::10/80

[Network]
IPv6AcceptRA=true
DNS=8.8.8.8
DNS=9.9.9.9

`))
		})

		It("configures postup routes for static network", func() {
			static1Net := boshsettings.Network{
				Type:    "manual",
				IP:      "2601:646:100:e8e8::103",
				Netmask: "ffff:ffff:ffff:ffff:0000:0000:0000:0000",
				Gateway: "2601:646:100:e8e8::",
				Default: []string{"gateway", "dns"},
				DNS:     []string{"8.8.8.8", "9.9.9.9"},
				Mac:     "mac1",
				Routes: []boshsettings.Route{
					boshsettings.Route{
						Destination: "2001:db8:1234::",
						Gateway:     "2601:646:100:e8e8::",
						Netmask:     "ffff:ffff:ffff:0000:0000:0000:0000:0000",
					},
				},
			}

			stubInterfaces(map[string]boshsettings.Network{
				"ethstatic1": static1Net,
			})

			err := netManager.SetupNetworking(boshsettings.Networks{
				"net1": static1Net,
			}, nil)
			Expect(err).ToNot(HaveOccurred())

			matches, err := fs.Ls("/etc/systemd/network/")
			Expect(err).NotTo(HaveOccurred())
			Expect(matches).To(ConsistOf(
				"/etc/systemd/network/10_ethstatic1.network",
			))
			networkConfig := fs.GetFileTestStat("/etc/systemd/network/10_ethstatic1.network")
			Expect(networkConfig).ToNot(BeNil())
			Expect(networkConfig.StringContents()).To(Equal(`# Generated by bosh-agent
[Match]
Name=ethstatic1

[Address]
Address=2601:646:100:e8e8::103/64

[Network]
Gateway=2601:646:100:e8e8::
IPv6AcceptRA=true
DNS=8.8.8.8
DNS=9.9.9.9

[Route]
Destination=2001:db8:1234::/48
Gateway=2601:646:100:e8e8::

`))
		})
	})
})
