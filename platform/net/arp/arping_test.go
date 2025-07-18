package arp_test

import (
	"errors"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/v2/platform/net/arp"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
)

type failingInterfaceAddress struct{}

func (ia failingInterfaceAddress) GetInterfaceName() string { return "eth0" }

func (ia failingInterfaceAddress) GetIP(ipProtocol boship.IPProtocol) (string, error) {
	return "", errors.New("fake-get-ip-err")
}

var _ = Describe("arping", func() {
	const arpingIterations = 6

	var (
		fs        *fakesys.FakeFileSystem
		cmdRunner *fakesys.FakeCmdRunner
		arping    AddressBroadcaster
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		cmdRunner = fakesys.NewFakeCmdRunner()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		arping = NewArping(cmdRunner, fs, logger, arpingIterations, 0, 0)
	})

	Describe("BroadcastMACAddresses", func() {
		BeforeEach(func() {
			err := fs.WriteFile("/sys/class/net/eth0", []byte{})
			Expect(err).NotTo(HaveOccurred())
			err = fs.WriteFile("/sys/class/net/eth1", []byte{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("runs arping commands for each interface", func() {
			addresses := []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "192.168.195.6"),
				boship.NewSimpleInterfaceAddress("eth1", "127.0.0.1"),
			}

			arping.BroadcastMACAddresses(addresses)

			countA := 0
			countB := 0

			a := []string{"arping", "-c", "1", "-U", "-I", "eth0", "192.168.195.6"}
			b := []string{"arping", "-c", "1", "-U", "-I", "eth1", "127.0.0.1"}

			for i := 0; i < arpingIterations*2; i++ {
				cmd := cmdRunner.RunCommands[i]
				if reflect.DeepEqual(cmd, a) {
					countA++
				} else if reflect.DeepEqual(cmd, b) {
					countB++
				} else {
					Fail("Unexpected command executed: " + strings.Join(cmd, " "))
				}
			}

			Expect(countA).To(Equal(arpingIterations))
			Expect(countB).To(Equal(arpingIterations))
		})

		It("does not run arping command if failed to get interface IP address", func() {
			addresses := []boship.InterfaceAddress{failingInterfaceAddress{}}

			arping.BroadcastMACAddresses(addresses)
			Expect(cmdRunner.RunCommands).To(BeEmpty())
		})

		It("ignores ipv6 addresses", func() {
			addresses := []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "fe80::"),
			}

			arping.BroadcastMACAddresses(addresses)
			Expect(len(cmdRunner.RunCommands)).To(Equal(0))
		})
	})
})
