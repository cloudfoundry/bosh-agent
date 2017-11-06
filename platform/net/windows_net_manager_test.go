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

		executeErr     error
		commandsLength int
		ccNetworks     boshsettings.Networks
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

		commandsLength = len(runner.RunCommands)
	})

	JustBeforeEach(func() {
		runner.AddCmdResult(fmt.Sprintf("-Command %s", GetIPv4InterfaceJSON),
			fakesys.FakeCmdResult{Stdout: interfacesJSON})

		// Allow 5 seconds to pass so that the Sleep() in the function can pass.
		go clock.WaitForWatcherAndIncrement(5 * time.Second)
		executeErr = netManager.SetupNetworking(ccNetworks, nil)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("Setting NIC settings", func() {
		Context("when applying settings succeeds", func() {
			BeforeEach(func() {
				macAddressDetector.setupMACs(network1, network2)
				ccNetworks = boshsettings.Networks{"net1": network1, "net2": network2, "vip": vip}
			})

			It("sets the IP address and netmask on all interfaces, and the gateway on the default gateway interface", func() {
				Expect(executeErr).ToNot(HaveOccurred())

				Expect(runner.RunCommands).To(
					ContainElement([]string{"-Command", fmt.Sprintf(NicSettingsTemplate, network1.Mac, network1.IP, network1.Netmask, network1.Gateway)}))
				Expect(runner.RunCommands).To(
					ContainElement([]string{"-Command", fmt.Sprintf(NicSettingsTemplate, network2.Mac, network2.IP, network2.Netmask, "")}))
			})

			Context("when VIP networks are present", func() {
				BeforeEach(func() {
					ccNetworks = boshsettings.Networks{"vip": vip}
				})

				It("ignores VIP networks", func() {
					Expect(executeErr).ToNot(HaveOccurred())
					Expect(len(runner.RunCommands)).To(Equal(commandsLength), fmt.Sprintf("Unexpected command(s) were run: %v", runner.RunCommands[commandsLength:]))
				})
			})
		})

		Context("when configuring fails", func() {
			BeforeEach(func() {
				ccNetworks = boshsettings.Networks{"static-1": network1}
				runner.AddCmdResult(
					"-Command "+fmt.Sprintf(NicSettingsTemplate, network1.Mac, network1.IP, network1.Netmask, network1.Gateway),
					fakesys.FakeCmdResult{Error: errors.New("fake-err")},
				)
			})

			It("returns an error when configuring fails", func() {
				Expect(executeErr).To(HaveOccurred())
				Expect(executeErr.Error()).To(Equal("Configuring interface: fake-err"))
			})
		})
	})

	Context("when there is a network marked default for DNS", func() {
		Context("when the command succeeds", func() {
			Context("when adding a single DNS server to the end of IPv4 DNS host lists", func() {
				BeforeEach(func() {
					network := boshsettings.Network{
						Type:    "manual",
						DNS:     []string{"8.8.8.8"},
						Default: []string{"gateway", "dns"},
					}
					ccNetworks = boshsettings.Networks{"net1": network}
				})

				It("succeeds", func() {
					Expect(executeErr).ToNot(HaveOccurred())

					Expect(runner.RunCommands).To(ContainElement(
						[]string{"-Command", GetIPv4InterfaceJSON}))
					Expect(runner.RunCommands).To(ContainElement(
						[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Ethernet", "169.254.0.2,10.0.0.1,8.8.8.8")}))
					Expect(runner.RunCommands).To(ContainElement(
						[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Loopback Pseudo-Interface 1", "8.8.8.8")}))
				})
			})

			Context("when adding multiple DNS servers to the end of IPv4 DNS host lists", func() {
				BeforeEach(func() {
					macAddressDetector.setupMACs(network1, network2)
					network := boshsettings.Network{
						Type:    "manual",
						DNS:     []string{"127.0.0.1", "169.254.0.2"},
						Default: []string{"gateway", "dns"},
					}
					ccNetworks = boshsettings.Networks{"net1": network}
				})

				It("appends non-pre-existing DNS hosts to the IPv4 DNS host lists", func() {
					Expect(executeErr).ToNot(HaveOccurred())

					Expect(runner.RunCommands).To(ContainElement(
						[]string{"-Command", GetIPv4InterfaceJSON}))
					Expect(runner.RunCommands).To(ContainElement(
						[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Ethernet", "169.254.0.2,10.0.0.1,127.0.0.1")}))
					Expect(runner.RunCommands).To(ContainElement(
						[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Loopback Pseudo-Interface 1", "127.0.0.1,169.254.0.2")}))
				})
			})

			Context("when the DNS host list is empty in the cloud config", func() {
				BeforeEach(func() {
					network := boshsettings.Network{
						Type:    "manual",
						Default: []string{"gateway", "dns"},
					}
					ccNetworks = boshsettings.Networks{"net1": network}
				})

				It("does nothing", func() {
					Expect(executeErr).ToNot(HaveOccurred())

					Expect(len(runner.RunCommands)).To(Equal(commandsLength), fmt.Sprintf("Unexpected command(s) were run: %v", runner.RunCommands[commandsLength:]))
				})
			})
		})

		Context("when an error is encountered", func() {
			BeforeEach(func() {
				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"127.0.0.1"},
					Default: []string{"gateway", "dns"},
				}
				ccNetworks = boshsettings.Networks{"static-1": network}
			})

			Context("and malformed JSON is returned when fetching the list of interfaces", func() {
				BeforeEach(func() {
					interfacesJSON = `gif89a[ { "InterfaceAlias": "Ethernet", "ServerAddresses": [`
				})

				It("returns an error", func() {
					Expect(executeErr).To(HaveOccurred())
				})
			})

			Context("when configuring DNS servers fails", func() {
				BeforeEach(func() {
					macAddressDetector.setupMACs(network1)

					runner.AddCmdResult(fmt.Sprintf("-Command %s", GetIPv4InterfaceJSON),
						fakesys.FakeCmdResult{Stdout: interfacesJSON})
					runner.AddCmdResult(
						"-Command "+fmt.Sprintf(SetInterfaceHostListTemplate, "Ethernet", "169.254.0.2,10.0.0.1,127.0.0.1"),
						fakesys.FakeCmdResult{Error: errors.New("fake-err")})
				})

				It("returns error if configuring DNS servers fails", func() {
					Expect(executeErr).To(HaveOccurred())
					Expect(executeErr.Error()).To(Equal("Setting DNS servers: fake-err"))
					Expect(runner.RunCommands).To(ContainElement(
						[]string{"-Command", GetIPv4InterfaceJSON}))
					Expect(runner.RunCommands).To(ContainElement(
						[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Ethernet", "169.254.0.2,10.0.0.1,127.0.0.1")}))
				})
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

		Context("when there is only one network", func() {
			BeforeEach(func() {
				network := boshsettings.Network{
					Type:    "manual",
					DNS:     []string{"127.0.0.1", "8.8.8.8"},
					Default: []string{"gateway"},
				}
				ccNetworks = boshsettings.Networks{"static-1": network}
			})

			It("configures DNS with DNS servers", func() {
				Expect(runner.RunCommands).To(ConsistOf(
					[]string{"-Command", fmt.Sprintf(SetInterfaceHostListTemplate, "Ethernet", "169.254.0.2,10.0.0.1,127.0.0.1,8.8.8.8")},
					[]string{"-Command", GetIPv4InterfaceJSON}))
			})
		})

		Context("when the DNS host list is empty in the cloud config and there are multiple networks", func() {
			BeforeEach(func() {
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
				ccNetworks = boshsettings.Networks{"man-1": testNetwork1, "man-2": testNetwork2}
			})

			It("does nothing", func() {
				Expect(executeErr).ToNot(HaveOccurred())

				Expect(len(runner.RunCommands)).To(Equal(commandsLength), fmt.Sprintf("Unexpected command(s) were run: %v", runner.RunCommands[commandsLength:]))
			})
		})
	})

	Context("when there is no non-vip network marked default for DNS", func() {
		BeforeEach(func() {
			network1 := boshsettings.Network{
				Type:    "manual",
				Default: []string{"gateway"},
			}

			network2 := boshsettings.Network{
				Type:    "vip",
				DNS:     []string{"8.8.8.8"},
				Default: []string{"gateway", "dns"},
			}

			ccNetworks = boshsettings.Networks{"static-1": network1, "vip-1": network2}
		})

		It("does nothing if the DNS host list is empty in the cloud config", func() {
			Expect(executeErr).ToNot(HaveOccurred())

			Expect(len(runner.RunCommands)).To(Equal(commandsLength), fmt.Sprintf("Unexpected command(s) were run: %v", runner.RunCommands[commandsLength:]))
		})
	})

	Context("when there are no networks", func() {
		It("does nothing", func() {
			Expect(executeErr).ToNot(HaveOccurred())

			Expect(len(runner.RunCommands)).To(Equal(commandsLength), fmt.Sprintf("Unexpected command(s) were run: %v", runner.RunCommands[commandsLength:]))
		})
	})
})
