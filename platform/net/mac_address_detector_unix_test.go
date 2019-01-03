package net_test

import (
	"errors"
	"fmt"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("MacAddressDetectorLinux", func() {
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
		fs.WriteFile(interfacePath, []byte{})
		if isPhysical {
			fs.WriteFile(fmt.Sprintf("/sys/class/net/%s/device", iface), []byte{})
		}
		fs.WriteFileString(fmt.Sprintf("/sys/class/net/%s/address", iface), fmt.Sprintf("%s\n", macAddress))
		fs.WriteFileString(fmt.Sprintf("/sys/class/net/%s/ifalias", iface), fmt.Sprintf("%s\n", ifalias))

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
		macAddressDetector = NewMacAddressDetector(fs)
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
