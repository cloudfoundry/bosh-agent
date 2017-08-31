package ip_test

import (
	"errors"
	gonet "net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net/ip"
	fakeip "github.com/cloudfoundry/bosh-agent/platform/net/ip/fakes"
)

var _ = Describe("simpleInterfaceAddress", func() {
	Describe("GetIP", func() {
		It("returns fully formatted IPv4", func() {
			ipStr, err := NewSimpleInterfaceAddress("iface", "127.0.0.1").GetIP()
			Expect(err).ToNot(HaveOccurred())
			Expect(ipStr).To(Equal("127.0.0.1"))
		})

		It("returns fully formatted IPv6", func() {
			ipStr, err := NewSimpleInterfaceAddress("iface", "ff00:f8::").GetIP()
			Expect(err).ToNot(HaveOccurred())
			Expect(ipStr).To(Equal("ff00:00f8:0000:0000:0000:0000:0000:0000"))

			ipStr, err = NewSimpleInterfaceAddress("iface", "1101:2202:3303:4404:5505:6606:7707:8808").GetIP()
			Expect(err).ToNot(HaveOccurred())
			Expect(ipStr).To(Equal("1101:2202:3303:4404:5505:6606:7707:8808"))
		})

		It("returns error if IP cannot be parsed", func() {
			_, err := NewSimpleInterfaceAddress("iface", "").GetIP()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Cannot parse IP ''"))
		})
	})
})

var _ = Describe("resolvingInterfaceAddress", func() {
	var (
		ipResolver       *fakeip.FakeResolver
		interfaceAddress InterfaceAddress
	)

	BeforeEach(func() {
		ipResolver = &fakeip.FakeResolver{}
		interfaceAddress = NewResolvingInterfaceAddress("fake-iface-name", ipResolver)
	})

	Describe("GetIP", func() {
		Context("when IP was not yet resolved", func() {
			BeforeEach(func() {
				ipResolver.GetPrimaryIPv4IPNet = &gonet.IPNet{
					IP:   gonet.ParseIP("127.0.0.1"),
					Mask: gonet.CIDRMask(16, 32),
				}
			})

			It("resolves the IP and returns fully formatted IPv4", func() {
				ip, err := interfaceAddress.GetIP()
				Expect(err).ToNot(HaveOccurred())
				Expect(ip).To(Equal("127.0.0.1"))

				Expect(ipResolver.GetPrimaryIPv4InterfaceName).To(Equal("fake-iface-name"))
			})

			It("resolves the IP and returns fully formatted IPv6", func() {
				ipResolver.GetPrimaryIPv4IPNet = &gonet.IPNet{
					IP:   gonet.ParseIP("ff00:f8::"),
					Mask: gonet.CIDRMask(64, 128),
				}

				ip, err := interfaceAddress.GetIP()
				Expect(err).ToNot(HaveOccurred())
				Expect(ip).To(Equal("ff00:00f8:0000:0000:0000:0000:0000:0000"))

				Expect(ipResolver.GetPrimaryIPv4InterfaceName).To(Equal("fake-iface-name"))
			})

			It("returns error if resolving IP fails", func() {
				ipResolver.GetPrimaryIPv4Err = errors.New("fake-get-primary-ipv4-err")

				ip, err := interfaceAddress.GetIP()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-primary-ipv4-err"))
				Expect(ip).To(Equal(""))
			})
		})

		Context("when IP was already resolved", func() {
			BeforeEach(func() {
				ipResolver.GetPrimaryIPv4IPNet = &gonet.IPNet{
					IP:   gonet.ParseIP("127.0.0.1"),
					Mask: gonet.CIDRMask(16, 32),
				}

				_, err := interfaceAddress.GetIP()
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not attempt to resolve IP again", func() {
				ipResolver.GetPrimaryIPv4InterfaceName = ""

				ip, err := interfaceAddress.GetIP()
				Expect(err).ToNot(HaveOccurred())
				Expect(ip).To(Equal("127.0.0.1"))

				Expect(ipResolver.GetPrimaryIPv4InterfaceName).To(Equal(""))
			})
		})
	})
})
