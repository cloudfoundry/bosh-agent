package net_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"

	. "github.com/cloudfoundry/bosh-agent/v2/platform/net"
	"github.com/cloudfoundry/bosh-agent/v2/platform/net/netfakes"
)

var _ = Describe("WindowsNetManager", func() {
	var (
		clock                         *fakeclock.FakeClock
		runner                        *fakesys.FakeCmdRunner
		netManager                    Manager
		interfaceConfigurationCreator InterfaceConfigurationCreator
		fs                            boshsys.FileSystem
		dirProvider                   boshdirs.Provider
		tmpDir                        string
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
		fakeMACAddressDetector = &netfakes.FakeMACAddressDetector{}
		runner = fakesys.NewFakeCmdRunner()
		clock = fakeclock.NewFakeClock(time.Now())
		logger := boshlog.NewLogger(boshlog.LevelNone)
		interfaceConfigurationCreator = NewInterfaceConfigurationCreator(logger)
		fs = boshsys.NewOsFileSystem(logger)

		var err error
		tmpDir, err = os.MkdirTemp("", "bosh-tests-")
		Expect(err).ToNot(HaveOccurred())
		dirProvider = boshdirs.NewProvider(tmpDir)
		err = fs.MkdirAll(filepath.Join(dirProvider.BoshDir()), 0755) //nolint:gocritic
		Expect(err).ToNot(HaveOccurred())

		netManager = NewWindowsNetManager(
			runner,
			interfaceConfigurationCreator,
			fakeMACAddressDetector,
			logger,
			clock,
			fs,
			dirProvider,
		)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir) //nolint:errcheck
	})

	setupNetworking := func(networks boshsettings.Networks) error {
		// Allow 5 seconds to pass so that the Sleep() in the function can pass.
		go clock.WaitForWatcherAndIncrement(5 * time.Second)
		return netManager.SetupNetworking(networks, "", nil)
	}

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
			stubInterfaces(map[string]boshsettings.Network{
				"net1": network1,
				"net2": network2,
			})
			err := setupNetworking(boshsettings.Networks{"net1": network1, "net2": network2, "vip": vip})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(ContainElement([]string{
				"netsh", "interface", "ip", "set", "address",
				`name=net1`, "source=static", "addr=192.168.50.50", "mask=255.255.255.0", "gateway=192.168.50.0", "gwmetric=1",
			}))
			Expect(runner.RunCommands).To(ContainElement([]string{
				"netsh", "interface", "ip", "set", "address",
				`name=net2`, "source=static", "addr=192.168.20.20", "mask=255.255.255.0", "gateway=none",
			}))
		})

		It("ignores VIP networks", func() {
			err := setupNetworking(boshsettings.Networks{"vip": vip})
			Expect(err).ToNot(HaveOccurred())
			Expect(runner.RunCommands).To(ContainElement([]string{
				"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
				"Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ResetServerAddresses }",
			}))
		})

		It("returns an error when configuring fails", func() {
			stubInterfaces(map[string]boshsettings.Network{
				"static-1": network1,
			})
			netshCmd := strings.Join([]string{
				"netsh", "interface", "ip", "set", "address",
				`name=static-1`, "source=static", "addr=192.168.50.50", "mask=255.255.255.0", "gateway=192.168.50.0", "gwmetric=1",
			}, " ")
			runner.AddCmdResult(
				netshCmd,
				fakesys.FakeCmdResult{Error: errors.New("fake-err")},
			)

			err := setupNetworking(boshsettings.Networks{"static-1": network1})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`Configuring interface "static-1"`))
			Expect(err.Error()).To(ContainSubstring("fake-err"))

			lockFile := filepath.Join(dirProvider.BoshDir(), "configured_interfaces.txt")
			Expect(lockFile).ToNot(BeAnExistingFile())
		})
	})

	Describe("lock file", func() {
		var network boshsettings.Network

		BeforeEach(func() {
			network = boshsettings.Network{
				Type:    "manual",
				DNS:     []string{"8.8.8.8"},
				Default: []string{"gateway", "dns"},
			}

			stubInterfaces(map[string]boshsettings.Network{
				"net1": network,
			})
		})

		Context("when the lock file exists", func() {
			BeforeEach(func() {
				lockFile := filepath.Join(dirProvider.BoshDir(), "dns")

				_, err := fs.OpenFile(lockFile, os.O_CREATE, 0644)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not configure DNS", func() {
				err := setupNetworking(boshsettings.Networks{"net1": network})
				Expect(err).ToNot(HaveOccurred())

				Expect(runner.RunCommands).NotTo(ContainElement([]string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Service http"}))
				for _, cmd := range runner.RunCommands {
					Expect(strings.Join(cmd, " ")).NotTo(ContainSubstring("Set-DnsClientServerAddress"))
				}
			})
		})

		Context("when the lock file does not exist", func() {
			It("configures DNS", func() {
				err := setupNetworking(boshsettings.Networks{"net1": network})
				Expect(err).ToNot(HaveOccurred())

				Expect(runner.RunCommands).To(ContainElement([]string{
					"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
					`$dns=@('8.8.8.8'); Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ServerAddresses $dns }`,
				}))

				dnsLockFile := filepath.Join(dirProvider.BoshDir(), "dns")
				Expect(dnsLockFile).To(BeAnExistingFile())
			})
		})
	})

	Context("when there is a network marked default for DNS", func() {
		It("configures DNS with a single DNS server", func() {
			network := boshsettings.Network{
				Type:    "manual",
				DNS:     []string{"8.8.8.8"},
				Default: []string{"gateway", "dns"},
			}

			stubInterfaces(map[string]boshsettings.Network{
				"net1": network,
			})

			err := setupNetworking(boshsettings.Networks{"net1": network})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(ContainElement([]string{
				"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
				`$dns=@('8.8.8.8'); Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ServerAddresses $dns }`,
			}))
		})

		It("configures DNS with multiple DNS servers", func() {
			network := boshsettings.Network{
				Type:    "manual",
				DNS:     []string{"127.0.0.1", "8.8.8.8"},
				Default: []string{"gateway", "dns"},
			}

			stubInterfaces(map[string]boshsettings.Network{
				"net1": network,
			})

			err := setupNetworking(boshsettings.Networks{"manual-1": network})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(ContainElement([]string{
				"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
				`$dns=@('127.0.0.1','8.8.8.8'); Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ServerAddresses $dns }`,
			}))
		})

		It("returns error when a DNS server is not a valid IP and does not run DNS set PowerShell", func() {
			network := boshsettings.Network{
				Type:    "manual",
				IP:      "192.168.50.50",
				Gateway: "192.168.50.0",
				Netmask: "255.255.255.0",
				Mac:     "00:0C:29:0B:69:7A",
				DNS:     []string{"8.8.8"},
				Default: []string{"gateway", "dns"},
			}

			stubInterfaces(map[string]boshsettings.Network{
				"net1": network,
			})

			err := setupNetworking(boshsettings.Networks{"net1": network})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`"8.8.8"`))

			for _, cmd := range runner.RunCommands {
				Expect(strings.Join(cmd, " ")).NotTo(ContainSubstring("$dns=@"))
			}

			dnsLockFile := filepath.Join(dirProvider.BoshDir(), "dns")
			_, statErr := os.Stat(dnsLockFile)
			Expect(statErr).To(HaveOccurred())
			Expect(os.IsNotExist(statErr)).To(BeTrue())
		})

		It("rejects IPv6 DNS server addresses for this configuration path", func() {
			network := boshsettings.Network{
				Type:    "manual",
				IP:      "192.168.50.50",
				Gateway: "192.168.50.0",
				Netmask: "255.255.255.0",
				Mac:     "00:0C:29:0B:69:7A",
				DNS:     []string{"2001:db8::1"},
				Default: []string{"gateway", "dns"},
			}

			stubInterfaces(map[string]boshsettings.Network{
				"net1": network,
			})

			err := setupNetworking(boshsettings.Networks{"net1": network})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("IPv6 is not supported"))
		})

		It("resets DNS without any DNS servers", func() {
			network := boshsettings.Network{
				Type:    "manual",
				Default: []string{"gateway", "dns"},
			}

			stubInterfaces(map[string]boshsettings.Network{
				"static-1": network,
			})

			err := setupNetworking(boshsettings.Networks{"static-1": network})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(ContainElement([]string{
				"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
				"Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ResetServerAddresses }",
			}))
		})

		It("returns error if configuring DNS servers fails", func() {
			network := boshsettings.Network{
				Type:    "manual",
				DNS:     []string{"127.0.0.1", "8.8.8.8"},
				Default: []string{"gateway", "dns"},
			}

			stubInterfaces(map[string]boshsettings.Network{
				"static-1": network,
			})

			runner.AddCmdResult(
				strings.Join([]string{
					"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
					`$dns=@('127.0.0.1','8.8.8.8'); Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ServerAddresses $dns }`,
				}, " "),
				fakesys.FakeCmdResult{Error: errors.New("fake-err")},
			)
			err := setupNetworking(boshsettings.Networks{"static-1": network})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Setting DNS servers: fake-err"))
		})

		It("returns error if resetting DNS servers fails", func() {
			network := boshsettings.Network{Type: "manual"}

			stubInterfaces(map[string]boshsettings.Network{
				"static-1": network,
			})

			runner.AddCmdResult(
				strings.Join([]string{
					"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
					"Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ResetServerAddresses }",
				}, " "),
				fakesys.FakeCmdResult{Error: errors.New("fake-err")},
			)

			err := setupNetworking(boshsettings.Networks{"static-1": network})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Setting DNS servers: fake-err"))
		})
	})

	Context("when there is no network marked default for DNS", func() {
		It("configures DNS with DNS servers if there is only one network", func() {
			network := boshsettings.Network{
				Type:    "manual",
				DNS:     []string{"127.0.0.1", "8.8.8.8"},
				Default: []string{"gateway"},
			}

			stubInterfaces(map[string]boshsettings.Network{
				"static-1": network,
			})

			err := setupNetworking(boshsettings.Networks{"static-1": network})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(ContainElement([]string{
				"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
				`$dns=@('127.0.0.1','8.8.8.8'); Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ServerAddresses $dns }`,
			}))
		})

		It("resets DNS without any DNS servers if there are multiple networks", func() {
			network1 := boshsettings.Network{
				Type:    "manual",
				Mac:     "aa:bb",
				DNS:     []string{"8.8.8.8"},
				Default: []string{"gateway"},
			}

			network2 := boshsettings.Network{
				Type:    "manual",
				Mac:     "dd:ee",
				DNS:     []string{"8.8.8.8"},
				Default: []string{"gateway"},
			}
			stubInterfaces(map[string]boshsettings.Network{
				"man-1": network1,
				"man-2": network2,
			})

			err := setupNetworking(boshsettings.Networks{"man-1": network1, "man-2": network2})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(ContainElement([]string{
				"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
				"Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ResetServerAddresses }",
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

			stubInterfaces(map[string]boshsettings.Network{
				"static-1": network1,
				"vip-1":    network2,
			})

			err := setupNetworking(boshsettings.Networks{"static-1": network1, "vip-1": network2})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(ContainElement([]string{
				"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
				"Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ResetServerAddresses }",
			}))
		})
	})

	Context("when there are no networks", func() {
		It("resets DNS", func() {
			err := setupNetworking(boshsettings.Networks{})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(ContainElement([]string{
				"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
				"Get-DNSClientServerAddress | ForEach-Object { Set-DnsClientServerAddress -InterfaceAlias $_.InterfaceAlias -ResetServerAddresses }",
			}))
		})
	})

	Describe("Setting HTTP Service", func() {
		Context("when calling SetupNetworking", func() {
			It("starts the HTTP service using Set-Service", func() {
				err := setupNetworking(boshsettings.Networks{})
				Expect(err).ToNot(HaveOccurred())

				Expect(runner.RunCommands).To(ContainElement([]string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Service http"}))
			})
		})
	})

	Describe("ValidateWindowsNetshInterfaceAlias", func() {
		It("allows typical Windows interface aliases", func() {
			Expect(ValidateWindowsNetshInterfaceAlias("Ethernet")).To(Succeed())
			Expect(ValidateWindowsNetshInterfaceAlias("Ethernet 2")).To(Succeed())
		})

		It("returns error when the name is empty or whitespace-only", func() {
			Expect(ValidateWindowsNetshInterfaceAlias("")).To(MatchError(ContainSubstring("empty")))
			Expect(ValidateWindowsNetshInterfaceAlias("   ")).To(MatchError(ContainSubstring("empty")))
		})

		It("returns error when the name contains double quotes or newlines", func() {
			Expect(ValidateWindowsNetshInterfaceAlias(`eth"bad`)).To(MatchError(ContainSubstring("double quotes")))
			Expect(ValidateWindowsNetshInterfaceAlias("eth\nbad")).To(MatchError(ContainSubstring("newlines")))
		})
	})
})
