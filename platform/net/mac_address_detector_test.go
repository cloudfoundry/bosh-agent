package net_test

import (
	"errors"
	"fmt"
	gonet "net"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("MacAddressDetector", func() {
	Describe("MacAddressDetectorLinux", func() {
		var (
			fs                 *fakesys.FakeFileSystem
			macAddressDetector MACAddressDetector
		)

		BeforeEach(func() {
			if runtime.GOOS == "windows" {
				Skip("Only run on unix")
			}
		})

		writeNetworkDevice := func(iface string, macAddress string, isPhysical bool, ifalias string) string {
			interfacePath := fmt.Sprintf("/sys/class/net/%s", iface)
			err := fs.WriteFile(interfacePath, []byte{})
			Expect(err).NotTo(HaveOccurred())
			if isPhysical {
				err = fs.WriteFile(fmt.Sprintf("/sys/class/net/%s/device", iface), []byte{})
				Expect(err).NotTo(HaveOccurred())
			}
			err = fs.WriteFileString(fmt.Sprintf("/sys/class/net/%s/address", iface), fmt.Sprintf("%s\n", macAddress))
			Expect(err).NotTo(HaveOccurred())
			err = fs.WriteFileString(fmt.Sprintf("/sys/class/net/%s/ifalias", iface), fmt.Sprintf("%s\n", ifalias))
			Expect(err).NotTo(HaveOccurred())

			return interfacePath
		}

		stubInterfacesWithVirtual := func(physicalInterfaces map[string]string, nonBoshManagedVirtualInterfaces map[string]string, boshManagedVirtualInterfaces map[string]string) {
			interfacePaths := []string{}

			for mac, iface := range physicalInterfaces {
				interfacePaths = append(interfacePaths, writeNetworkDevice(iface, mac, true, ""))
			}

			for mac, iface := range nonBoshManagedVirtualInterfaces {
				interfacePaths = append(interfacePaths, writeNetworkDevice(iface, mac, false, ""))
			}

			for mac, iface := range boshManagedVirtualInterfaces {
				interfacePaths = append(interfacePaths, writeNetworkDevice(iface, mac, false, fmt.Sprintf("bosh-interface-%s", iface)))
			}

			fs.SetGlob("/sys/class/net/*", interfacePaths)
		}

		BeforeEach(func() {
			fs = fakesys.NewFakeFileSystem()
			macAddressDetector = NewLinuxMacAddressDetector(fs)
		})

		Describe("DetectMacAddresses", func() {
			Context("when there are only physical interfaces", func() {
				It("should detect all interfaces", func() {
					stubInterfacesWithVirtual(map[string]string{
						"aa:bb": "eth0",
						"cc:dd": "eth1",
					}, nil, nil)
					interfacesByMacAddress, err := macAddressDetector.DetectMacAddresses()
					Expect(err).ToNot(HaveOccurred())
					Expect(interfacesByMacAddress).To(Equal(map[string]string{
						"aa:bb": "eth0",
						"cc:dd": "eth1",
					}))
				})
			})

			Context("when there are physical interfaces and virtual interfaces", func() {
				It("should detect all physical interfaces and virtual interfaces that have bosh ifalias", func() {
					stubInterfacesWithVirtual(map[string]string{
						"aa:bb": "eth0",
						"cc:dd": "eth1",
					}, map[string]string{
						"11:22": "veth0",
					}, map[string]string{
						"33:44": "veth2",
					})
					interfacesByMacAddress, err := macAddressDetector.DetectMacAddresses()
					Expect(err).ToNot(HaveOccurred())
					Expect(interfacesByMacAddress).To(Equal(map[string]string{
						"aa:bb": "eth0",
						"cc:dd": "eth1",
						"33:44": "veth2",
					}))
				})
			})

			It("returns errors from glob /sys/class/net/", func() {
				fs.GlobErr = errors.New("fs-glob-error")
				_, err := macAddressDetector.DetectMacAddresses()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fs-glob-error"))
			})
		})
	})

	Describe("WindowsMacAddressDetector", func() {
		var (
			macAddress                gonet.HardwareAddr
			macAddressDetector        MACAddressDetector
			runner                    *fakesys.FakeCmdRunner
			interfacesFunctionReturns []gonet.Interface
			interfacesFunctionError   error
		)

		BeforeEach(func() {
			runner = fakesys.NewFakeCmdRunner()
			macAddress, _ = gonet.ParseMAC("12:34:56:78:9a:bc")
			fakeInterfacesFunction := func() ([]gonet.Interface, error) {
				if interfacesFunctionError != nil {
					return nil, interfacesFunctionError
				}
				return interfacesFunctionReturns, interfacesFunctionError
			}
			interfacesFunctionReturns = []gonet.Interface{}
			interfacesFunctionError = nil
			macAddressDetector = NewWindowsMacAddressDetector(runner, fakeInterfacesFunction)
		})

		Context("when only one adapter exists", func() {
			BeforeEach(func() {
				interfacesFunctionReturns = []gonet.Interface{{Name: "Ethernet0", HardwareAddr: macAddress}}
				runner.AddCmdResult(
					"powershell -Command Get-NetAdapter | Select MacAddress,Name | ConvertTo-Json",
					fakesys.FakeCmdResult{Stdout: `{
						"MacAddress":  "12-34-56-78-9A-BC",
						"Name":  "Ethernet0"
				}`},
				)
			})

			It("returns info for the only adapter", func() {
				macNameMap, err := macAddressDetector.DetectMacAddresses()

				Expect(err).ToNot(HaveOccurred())
				Expect(macNameMap).To(Equal(map[string]string{"12:34:56:78:9a:bc": "Ethernet0"}))
			})
		})

		Context("when the adapter is replaced with a vEthernet adapter, and other hidden vEthernet adapters with the same MAC exist", func() {
			BeforeEach(func() {
				interfacesFunctionReturns = []gonet.Interface{{Name: "vEthernet (Ethernet0)", HardwareAddr: macAddress}, {Name: "vEthernet (agent0)", HardwareAddr: macAddress}}
				runner.AddCmdResult(
					"powershell -Command Get-NetAdapter | Select MacAddress,Name | ConvertTo-Json",
					fakesys.FakeCmdResult{Stdout: `[
						{
							"MacAddress":  "12-34-56-78-9A-BC",
							"Name":  "Ethernet0"
						},
						{
							"MacAddress":  "12-34-56-78-9A-BC",
							"Name":  "vEthernet (Ethernet0)"
						}
					]`},
				)
			})

			It("returns info for the only adapter", func() {
				macNameMap, err := macAddressDetector.DetectMacAddresses()

				Expect(err).ToNot(HaveOccurred())
				Expect(macNameMap).To(Equal(map[string]string{"12:34:56:78:9a:bc": "vEthernet (Ethernet0)"}))
			})
		})

		Context("when executing Get-NetAdapter fails", func() {
			BeforeEach(func() {
				interfacesFunctionReturns = []gonet.Interface{{Name: "Ethernet0", HardwareAddr: macAddress}}
				runner.AddCmdResult(
					"powershell -Command Get-NetAdapter | Select MacAddress,Name | ConvertTo-Json",
					fakesys.FakeCmdResult{Error: errors.New("Getting NetAdapters is hard"), Stderr: "Donkey"},
				)
			})

			It("returns the error and stderr output", func() {
				_, err := macAddressDetector.DetectMacAddresses()

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Getting visible adapters"))
				Expect(err.Error()).To(ContainSubstring("Donkey"))
				Expect(err.Error()).To(ContainSubstring("Getting NetAdapters is hard"))
			})
		})

		Context("when Get-NetAdapter returns an unparsable MAC Address for an interface", func() {
			BeforeEach(func() {
				interfacesFunctionReturns = []gonet.Interface{{Name: "Ethernet0", HardwareAddr: macAddress}}
				runner.AddCmdResult(
					"powershell -Command Get-NetAdapter | Select MacAddress,Name | ConvertTo-Json",
					fakesys.FakeCmdResult{Stdout: "Monkey Banana"},
				)
			})

			It("returns the error", func() {
				_, err := macAddressDetector.DetectMacAddresses()

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Parsing Get-NetAdapter output"))
			})
		})

		Context("when executing the Interfaces function fails", func() {
			BeforeEach(func() {
				interfacesFunctionError = errors.New("Getting interfaces is hard")
			})

			It("returns the error", func() {
				_, err := macAddressDetector.DetectMacAddresses()

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Detecting Mac Addresses"))
				Expect(err.Error()).To(ContainSubstring("Getting interfaces is hard"))
			})
		})
	})
})
