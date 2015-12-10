package net_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("ubuntuNetManager", func() {
	var (
		psRunner   *fakesys.FakePSRunner
		netManager Manager
	)

	BeforeEach(func() {
		psRunner = fakesys.NewFakePSRunner()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		netManager = NewWindowsNetManager(psRunner, logger)
	})

	Describe("SetupNetworking", func() {
		Context("when there is a network marked default for DNS", func() {
			It("configures DNS with a single DNS server", func() {
				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"8.8.8.8"},
					Default: []string{"gateway", "dns"},
				}

				err := netManager.SetupNetworking(boshsettings.Networks{"net1": network}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(psRunner.RunCommands).To(Equal([]boshsys.PSCommand{
					{
						Script: `
[array]$interfaces = Get-DNSClientServerAddress
$dns = @("8.8.8.8")
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ServerAddresses ($dns -join ",")
}
`,
					},
				}))
			})

			It("configures DNS with multiple DNS servers", func() {
				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"127.0.0.1", "8.8.8.8"},
					Default: []string{"gateway", "dns"},
				}

				err := netManager.SetupNetworking(boshsettings.Networks{"manual-1": network}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(psRunner.RunCommands).To(Equal([]boshsys.PSCommand{
					{
						Script: `
[array]$interfaces = Get-DNSClientServerAddress
$dns = @("127.0.0.1","8.8.8.8")
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ServerAddresses ($dns -join ",")
}
`,
					},
				}))
			})

			It("resets DNS without any DNS servers", func() {
				network := boshsettings.Network{
					Type:    "manual",
					Default: []string{"gateway", "dns"},
				}

				err := netManager.SetupNetworking(boshsettings.Networks{"static-1": network}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(psRunner.RunCommands).To(Equal([]boshsys.PSCommand{
					{
						Script: `
[array]$interfaces = Get-DNSClientServerAddress
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ResetServerAddresses
}
`,
					},
				}))
			})

			It("returns error if configuring DNS servers fails", func() {
				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"127.0.0.1", "8.8.8.8"},
					Default: []string{"gateway", "dns"},
				}

				psRunner.RunCommandErr = errors.New("fake-err")

				err := netManager.SetupNetworking(boshsettings.Networks{"static-1": network}, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-err"))
			})

			It("returns error if resetting DNS servers fails", func() {
				network := boshsettings.Network{Type: "manual"}

				psRunner.RunCommandErr = errors.New("fake-err")

				err := netManager.SetupNetworking(boshsettings.Networks{"static-1": network}, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-err"))
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

				Expect(psRunner.RunCommands).To(Equal([]boshsys.PSCommand{
					{
						Script: `
[array]$interfaces = Get-DNSClientServerAddress
$dns = @("127.0.0.1","8.8.8.8")
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ServerAddresses ($dns -join ",")
}
`,
					},
				}))
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

				Expect(psRunner.RunCommands).To(Equal([]boshsys.PSCommand{
					{
						Script: `
[array]$interfaces = Get-DNSClientServerAddress
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ResetServerAddresses
}
`,
					},
				}))
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

				Expect(psRunner.RunCommands).To(Equal([]boshsys.PSCommand{
					{
						Script: `
[array]$interfaces = Get-DNSClientServerAddress
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ResetServerAddresses
}
`,
					},
				}))
			})
		})

		Context("when there are no networks", func() {
			It("resets DNS", func() {
				err := netManager.SetupNetworking(boshsettings.Networks{}, nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(psRunner.RunCommands).To(Equal([]boshsys.PSCommand{
					{
						Script: `
[array]$interfaces = Get-DNSClientServerAddress
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ResetServerAddresses
}
`,
					},
				}))
			})
		})
	})
})
