package net_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	gonet "net"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/platform/net"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randomMAC() string {
	hw := make(gonet.HardwareAddr, 6)
	for i := 0; i < len(hw); i++ {
		hw[i] = byte(rand.Intn(1<<8 - 1))
	}
	return hw.String()
}

type fakeMACAddressDetector struct {
	macs map[string]string
}

func (m *fakeMACAddressDetector) MACAddresses() (map[string]string, error) {
	return m.macs, nil
}

func (m *fakeMACAddressDetector) setupMACs(networks ...boshsettings.Network) {
	myMap := make(map[string]string)
	for i, net := range networks {
		if net.Mac != "" {
			myMap[net.Mac] = fmt.Sprintf("Eth_HW %d", i)
		} else {
			myMap[randomMAC()] = fmt.Sprintf("Eth_Rand %d", i)
		}
	}
	m.macs = myMap
}

var _ = Describe("WindowsNetManager", func() {
	var (
		clock                         *fakeclock.FakeClock
		runner                        *fakesys.FakeCmdRunner
		netManager                    Manager
		interfaceConfigurationCreator InterfaceConfigurationCreator
		fs                            boshsys.FileSystem
		dirProvider                   boshdirs.Provider
		tmpDir                        string
		macAddressDetector            *fakeMACAddressDetector
		interfacesJSON                string

		network1 boshsettings.Network
		network2 boshsettings.Network
		vip      boshsettings.Network
	)

	BeforeEach(func() {
		network1 = boshsettings.Network{
			Type:    "manual",
			DNS:     []string{"8.8.8.8"},
			Default: []string{"gateway", "dns"},
			IP:      "192.168.50.50",
			Gateway: "192.168.50.0",
			Netmask: "255.255.255.0",
			Mac:     "00:0C:29:0B:69:7A",
		}
		network2 = boshsettings.Network{
			Type:    "manual",
			DNS:     []string{"8.8.8.8"},
			Default: []string{},
			IP:      "192.168.20.20",
			Gateway: "192.168.20.0",
			Netmask: "255.255.255.0",
			Mac:     "99:55:C3:5A:52:7A",
		}
		vip = boshsettings.Network{
			Type: "vip",
		}

		interfacesJSON = `[
												{
													"InterfaceAlias": "Ethernet",
													"ServerAddresses": [
														"169.254.0.2",
														"10.0.0.1"
													]
												},
												{
													"InterfaceAlias": "Loopback Pseudo-Interface 1",
													"ServerAddresses": []
												}
											]`

		macAddressDetector = new(fakeMACAddressDetector)
		macAddressDetector.setupMACs(network1)
		runner = fakesys.NewFakeCmdRunner()
		clock = fakeclock.NewFakeClock(time.Now())
		logger := boshlog.NewLogger(boshlog.LevelNone)
		interfaceConfigurationCreator = NewInterfaceConfigurationCreator(logger)
		fs = boshsys.NewOsFileSystem(logger)

		var err error
		tmpDir, err = ioutil.TempDir("", "bosh-tests-")
		Expect(err).ToNot(HaveOccurred())
		dirProvider = boshdirs.NewProvider(tmpDir)
		err = fs.MkdirAll(filepath.Join(dirProvider.BoshDir()), 0755)
		Expect(err).ToNot(HaveOccurred())

		netManager = NewWindowsNetManager(
			runner,
			interfaceConfigurationCreator,
			macAddressDetector,
			logger,
			clock,
			fs,
			dirProvider,
		)
	})

	JustBeforeEach(func() {
		runner.AddCmdResult(fmt.Sprintf("-Command %s", GetIPv4InterfaceJSON),
			fakesys.FakeCmdResult{Stdout: interfacesJSON})
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	setupNetworking := func(networks boshsettings.Networks) error {
		// Allow 5 seconds to pass so that the Sleep() in the function can pass.
		go clock.WaitForWatcherAndIncrement(5 * time.Second)
		return netManager.SetupNetworking(networks, nil)
	}

	Describe("Setting NIC settings", func() {
		It("sets the IP address and netmask on all interfaces, and the gateway on the default gateway interface", func() {
			macAddressDetector.setupMACs(network1, network2)
			err := setupNetworking(boshsettings.Networks{"net1": network1, "net2": network2, "vip": vip})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(
				ContainElement([]string{"-Command", fmt.Sprintf(NicSettingsTemplate, network1.Mac, network1.IP, network1.Netmask, network1.Gateway)}))
			Expect(runner.RunCommands).To(
				ContainElement([]string{"-Command", fmt.Sprintf(NicSettingsTemplate, network2.Mac, network2.IP, network2.Netmask, "")}))
		})

		It("ignores VIP networks", func() {
			macAddressDetector.setupMACs(network1, network2)
			err := setupNetworking(boshsettings.Networks{"vip": vip})
			Expect(err).ToNot(HaveOccurred())
			Expect(runner.RunCommands).To(Equal([][]string{[]string{"-Command", ResetDNSHostList}}))
		})

		It("returns an error when configuring fails", func() {
			runner.AddCmdResult(
				"-Command "+fmt.Sprintf(NicSettingsTemplate, network1.Mac, network1.IP, network1.Netmask, network1.Gateway),
				fakesys.FakeCmdResult{Error: errors.New("fake-err")},
			)

			err := setupNetworking(boshsettings.Networks{"static-1": network1})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Configuring interface: fake-err"))
		})
	})

	Context("when there is a network marked default for DNS", func() {
		Context("when the command succeeds", func() {
			It("adds a single DNS server to the end of IPv4 DNS host lists", func() {
				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"8.8.8.8"},
					Default: []string{"gateway", "dns"},
				}

				err := setupNetworking(boshsettings.Networks{"net1": network})
				Expect(err).ToNot(HaveOccurred())

				Expect(runner.RunCommands).To(ContainElement(
					[]string{"-Command", GetIPv4InterfaceJSON}))
				Expect(runner.RunCommands).To(ContainElement(
					[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Ethernet", "169.254.0.2,10.0.0.1,8.8.8.8")}))
				Expect(runner.RunCommands).To(ContainElement(
					[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Loopback Pseudo-Interface 1", "8.8.8.8")}))
			})

			It("appends non-pre-existing DNS hosts to the IPv4 DNS host lists", func() {
				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"127.0.0.1", "169.254.0.2"},
					Default: []string{"gateway", "dns"},
				}
				macAddressDetector.setupMACs(network1, network2)
				err := setupNetworking(boshsettings.Networks{"manual-1": network})
				Expect(err).ToNot(HaveOccurred())

				Expect(runner.RunCommands).To(ContainElement(
					[]string{"-Command", GetIPv4InterfaceJSON}))
				Expect(runner.RunCommands).To(ContainElement(
					[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Ethernet", "169.254.0.2,10.0.0.1,127.0.0.1")}))
				Expect(runner.RunCommands).To(ContainElement(
					[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Loopback Pseudo-Interface 1", "127.0.0.1,169.254.0.2")}))
			})

			It("resets DNS without any DNS servers", func() {
				network := boshsettings.Network{
					Type:    "manual",
					Default: []string{"gateway", "dns"},
				}

				err := setupNetworking(boshsettings.Networks{"static-1": network})
				Expect(err).ToNot(HaveOccurred())

				Expect(runner.RunCommands).To(ContainElement(
					[]string{"-Command", ResetDNSHostList}))
			})
		})

		Context("when an error is encountered", func() {
			Context("and if malformed JSON is returned when fetching the list of interfaces", func() {
				BeforeEach(func() {
					interfacesJSON = `gif89a[ { "InterfaceAlias": "Ethernet", "ServerAddresses": [`
				})

				It("returns an error", func() {
					network := boshsettings.Network{
						Type:    "manual",
						Default: []string{"gateway", "dns"},
						DNS:     []string{"127.0.0.1"},
					}

					err := setupNetworking(boshsettings.Networks{"static-1": network})
					Expect(err).To(HaveOccurred())
				})
			})

			It("returns error if configuring DNS servers fails", func() {
				macAddressDetector.setupMACs(network1)

				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"127.0.0.1"},
					Default: []string{"gateway", "dns"},
				}

				runner.AddCmdResult(fmt.Sprintf("-Command %s", GetIPv4InterfaceJSON),
					fakesys.FakeCmdResult{Stdout: interfacesJSON})
				runner.AddCmdResult(
					"-Command "+fmt.Sprintf(SetInterfaceHostListTemplate, "Ethernet", "169.254.0.2,10.0.0.1,127.0.0.1"),
					fakesys.FakeCmdResult{Error: errors.New("fake-err")})

				err := setupNetworking(boshsettings.Networks{"static-1": network})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Setting DNS servers: fake-err"))
				Expect(runner.RunCommands).To(ContainElement(
					[]string{"-Command", GetIPv4InterfaceJSON}))
				Expect(runner.RunCommands).To(ContainElement(
					[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Ethernet", "169.254.0.2,10.0.0.1,127.0.0.1")}))
			})

			It("returns error if resetting DNS servers fails", func() {
				network := boshsettings.Network{Type: "manual"}
				runner.AddCmdResult(fmt.Sprintf("-Command %s", GetIPv4InterfaceJSON),
					fakesys.FakeCmdResult{Stdout: interfacesJSON})

				runner.AddCmdResult(
					"-Command "+ResetDNSHostList,
					fakesys.FakeCmdResult{Error: errors.New("fake-err")},
				)

				err := setupNetworking(boshsettings.Networks{"static-1": network})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Setting DNS servers: fake-err"))
			})
		})
	})

	Context("when there is no network marked default for DNS", func() {
		BeforeEach(func() {
			interfacesJSON = `[
													{
														"InterfaceAlias": "Ethernet",
														"ServerAddresses": [
															"169.254.0.2",
															"10.0.0.1"
														]
													}
												]`
		})

		It("configures DNS with DNS servers if there is only one network", func() {
			network := boshsettings.Network{
				Type:    "manual",
				DNS:     []string{"127.0.0.1", "8.8.8.8"},
				Default: []string{"gateway"},
			}
			err := setupNetworking(boshsettings.Networks{"static-1": network})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(ConsistOf(
				[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Ethernet", "169.254.0.2,10.0.0.1,127.0.0.1,8.8.8.8")},
				[]string{"-Command", GetIPv4InterfaceJSON}))
		})

		It("resets DNS without any DNS servers if there are multiple networks", func() {
			testNetwork1 := boshsettings.Network{
				Type:    "manual",
				DNS:     []string{"8.8.8.8"},
				Default: []string{"gateway"},
			}

			testNetwork2 := boshsettings.Network{
				Type:    "manual",
				DNS:     []string{"8.8.8.8"},
				Default: []string{"gateway"},
			}

			macAddressDetector.setupMACs(testNetwork1, testNetwork2)
			err := setupNetworking(boshsettings.Networks{"man-1": testNetwork1, "man-2": testNetwork2})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(Equal([][]string{[]string{"-Command", ResetDNSHostList}}))
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

			err := setupNetworking(boshsettings.Networks{"static-1": network1, "vip-1": network2})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(Equal([][]string{[]string{"-Command", ResetDNSHostList}}))
		})
	})

	Context("when there are no networks", func() {
		It("resets DNS", func() {
			err := setupNetworking(boshsettings.Networks{})
			Expect(err).ToNot(HaveOccurred())

			Expect(runner.RunCommands).To(Equal([][]string{[]string{"-Command", ResetDNSHostList}}))
		})
	})
})
