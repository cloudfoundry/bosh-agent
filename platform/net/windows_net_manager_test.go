package net_test

import (
	"errors"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("WindowsNetManager", func() {
	var (
		scriptRunner *fakesys.FakeScriptRunner
		netManager   Manager
	)

	BeforeEach(func() {
		scriptRunner = fakesys.NewFakeScriptRunner()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		netManager = NewWindowsNetManager(scriptRunner, logger)
	})

	Describe("SetupNetworking", func() {

		Describe("Setting NIC settings", func() {
			network1 := boshsettings.Network{
				Type:    "manual",
				DNS:     []string{"8.8.8.8"},
				Default: []string{"gateway", "dns"},
				IP:      "192.168.50.50",
				Gateway: "192.168.50.0",
				Netmask: "255.255.255.0",
				Mac:     "00:0C:29:0B:69:7A",
			}

			network2 := boshsettings.Network{
				Type:    "manual",
				DNS:     []string{"8.8.8.8"},
				Default: []string{},
				IP:      "192.168.20.20",
				Gateway: "192.168.20.0",
				Netmask: "255.255.255.0",
				Mac:     "99:55:C3:5A:52:7A",
			}

			vip := boshsettings.Network{
				Type: "vip",
			}

			It("sets the IP address and netmask on all interfaces, and the gateway on the default gateway interface", func() {
				err := netManager.SetupNetworking(boshsettings.Networks{"net1": network1, "net2": network2, "vip": vip}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(scriptRunner.RunScripts).To(ContainElement(`
$connectionName=(get-wmiobject win32_networkadapter | where-object {$_.MacAddress -eq '00:0C:29:0B:69:7A'}).netconnectionid
netsh interface ip set address $connectionName static 192.168.50.50 255.255.255.0 192.168.50.0
`))
				Expect(scriptRunner.RunScripts).To(ContainElement(
					strings.Join([]string{
						"",
						"$connectionName=(get-wmiobject win32_networkadapter | where-object {$_.MacAddress -eq '99:55:C3:5A:52:7A'}).netconnectionid",
						"netsh interface ip set address $connectionName static 192.168.20.20 255.255.255.0 ",
						"",
					}, "\n")))
			})

			It("sets the gateway when there is only one network and it is not the default for gateway", func() {
				err := netManager.SetupNetworking(boshsettings.Networks{"net": network2, "vip": vip}, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(scriptRunner.RunScripts).To(ContainElement(`
$connectionName=(get-wmiobject win32_networkadapter | where-object {$_.MacAddress -eq '99:55:C3:5A:52:7A'}).netconnectionid
netsh interface ip set address $connectionName static 192.168.20.20 255.255.255.0 192.168.20.0
`))
			})

			It("ignores VIP networks", func() {
				err := netManager.SetupNetworking(boshsettings.Networks{"vip": vip}, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(scriptRunner.RunScripts).To(Equal([]string{`
[array]$interfaces = Get-DNSClientServerAddress
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ResetServerAddresses
}
`}))
			})

			It("returns an error when configuring fails", func() {
				scriptRunner.RegisterRunScriptError(fmt.Sprintf(NicSettingsTemplate, network1.Mac, network1.IP, network1.Netmask, network1.Gateway), errors.New("fake-err"))
				err := netManager.SetupNetworking(boshsettings.Networks{"static-1": network1}, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Configuring interface: fake-err"))
			})
		})

		Context("when there is a network marked default for DNS", func() {
			It("configures DNS with a single DNS server", func() {
				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"8.8.8.8"},
					Default: []string{"gateway", "dns"},
				}

				err := netManager.SetupNetworking(boshsettings.Networks{"net1": network}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(scriptRunner.RunScripts).To(ContainElement(`
[array]$interfaces = Get-DNSClientServerAddress
$dns = @("8.8.8.8")
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ServerAddresses ($dns -join ",")
}
`))
			})

			It("configures DNS with multiple DNS servers", func() {
				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"127.0.0.1", "8.8.8.8"},
					Default: []string{"gateway", "dns"},
				}

				err := netManager.SetupNetworking(boshsettings.Networks{"manual-1": network}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(scriptRunner.RunScripts).To(ContainElement(`
[array]$interfaces = Get-DNSClientServerAddress
$dns = @("127.0.0.1","8.8.8.8")
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ServerAddresses ($dns -join ",")
}
`))
			})

			It("resets DNS without any DNS servers", func() {
				network := boshsettings.Network{
					Type:    "manual",
					Default: []string{"gateway", "dns"},
				}

				err := netManager.SetupNetworking(boshsettings.Networks{"static-1": network}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(scriptRunner.RunScripts).To(ContainElement(`
[array]$interfaces = Get-DNSClientServerAddress
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ResetServerAddresses
}
`))
			})

			It("returns error if configuring DNS servers fails", func() {
				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"127.0.0.1", "8.8.8.8"},
					Default: []string{"gateway", "dns"},
				}

				scriptRunner.RegisterRunScriptError(fmt.Sprintf(SetDNSTemplate, strings.Join(network.DNS, `","`)), errors.New("fake-err"))

				err := netManager.SetupNetworking(boshsettings.Networks{"static-1": network}, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Configuring DNS servers: fake-err"))
			})

			It("returns error if resetting DNS servers fails", func() {
				network := boshsettings.Network{Type: "manual"}

				scriptRunner.RegisterRunScriptError(ResetDNSTemplate, errors.New("fake-err"))

				err := netManager.SetupNetworking(boshsettings.Networks{"static-1": network}, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Resetting DNS servers: fake-err"))
			})
		})

		Context("when there is no network marked default for DNS", func() {
			It("configures DNS with DNS servers if there is only one network", func() {
				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"127.0.0.1", "8.8.8.8"},
					Default: []string{"gateway"},
				}

				err := netManager.SetupNetworking(boshsettings.Networks{"static-1": network}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(scriptRunner.RunScripts).To(ContainElement(`
[array]$interfaces = Get-DNSClientServerAddress
$dns = @("127.0.0.1","8.8.8.8")
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ServerAddresses ($dns -join ",")
}
`))
			})

			It("resets DNS without any DNS servers if there are multiple networks", func() {
				network1 := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"8.8.8.8"},
					Default: []string{"gateway"},
				}

				network2 := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"8.8.8.8"},
					Default: []string{"gateway"},
				}

				err := netManager.SetupNetworking(boshsettings.Networks{"man-1": network1, "man-2": network2}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(scriptRunner.RunScripts).To(ContainElement(`
[array]$interfaces = Get-DNSClientServerAddress
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ResetServerAddresses
}
`))
			})
		})

		Context("when there is no non-vip network marked default for DNS", func() {
			It("resets DNS without any DNS servers", func() {
				network1 := boshsettings.Network{
					Type:    "manual",
					Default: []string{"gateway"},
				}

				network2 := boshsettings.Network{
					Type:    "vip",
					DNS:     []string{"8.8.8.8"},
					Default: []string{"gateway", "dns"},
				}

				err := netManager.SetupNetworking(boshsettings.Networks{"static-1": network1, "vip-1": network2}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(scriptRunner.RunScripts).To(ContainElement(`
[array]$interfaces = Get-DNSClientServerAddress
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ResetServerAddresses
}
`))
			})
		})

		Context("when there are no networks", func() {
			It("resets DNS", func() {
				err := netManager.SetupNetworking(boshsettings.Networks{}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(scriptRunner.RunScripts).To(Equal([]string{`
[array]$interfaces = Get-DNSClientServerAddress
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ResetServerAddresses
}
`}))
			})
		})
	})
})
