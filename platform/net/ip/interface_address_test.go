package ip_test

import (
	"errors"
	gonet "net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	fakeip "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip/fakes"
)

var _ = Describe("simpleInterfaceAddress", func() {
	Describe("GetIP", func() {
		It("returns fully formatted IPv4", func() {
			ipStr, err := NewSimpleInterfaceAddress("iface", "127.0.0.1").GetIP(IPv4)
			Expect(err).ToNot(HaveOccurred())
			Expect(ipStr).To(Equal("127.0.0.1"))
		})

		It("returns fully formatted IPv6", func() {
			ipStr, err := NewSimpleInterfaceAddress("iface", "ff00:f8::").GetIP(IPv6)
			Expect(err).ToNot(HaveOccurred())
			Expect(ipStr).To(Equal("ff00:00f8:0000:0000:0000:0000:0000:0000"))

			ipStr, err = NewSimpleInterfaceAddress("iface", "1101:2202:3303:4404:5505:6606:7707:8808").GetIP(IPv6)
			Expect(err).ToNot(HaveOccurred())
			Expect(ipStr).To(Equal("1101:2202:3303:4404:5505:6606:7707:8808"))
		})

		It("returns error if IP cannot be parsed", func() {
			_, err := NewSimpleInterfaceAddress("iface", "").GetIP(IPv4)
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
				ipResolver.GetPrimaryIPNet = &gonet.IPNet{
					IP:   gonet.ParseIP("127.0.0.1"),
					Mask: gonet.CIDRMask(16, 32),
				}
			})

			It("resolves the IP and returns fully formatted IPv4", func() {
				ip, err := interfaceAddress.GetIP(IPv4)
				Expect(err).ToNot(HaveOccurred())
				Expect(ip).To(Equal("127.0.0.1"))

				Expect(ipResolver.GetPrimaryIPInterfaceName).To(Equal("fake-iface-name"))
			})

			It("resolves the IP and returns fully formatted IPv6", func() {
				ipResolver.GetPrimaryIPNet = &gonet.IPNet{
					IP:   gonet.ParseIP("ff00:f8::"),
					Mask: gonet.CIDRMask(64, 128),
				}

				ip, err := interfaceAddress.GetIP(IPv4)
				Expect(err).ToNot(HaveOccurred())
				Expect(ip).To(Equal("ff00:00f8:0000:0000:0000:0000:0000:0000"))

				Expect(ipResolver.GetPrimaryIPInterfaceName).To(Equal("fake-iface-name"))
			})

			It("returns error if resolving IP fails", func() {
				ipResolver.GetPrimaryIPErr = errors.New("fake-get-primary-ipv4-err")

				ip, err := interfaceAddress.GetIP(IPv4)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-primary-ipv4-err"))
				Expect(ip).To(Equal(""))
			})
		})

		Context("when IP was already resolved", func() {
			BeforeEach(func() {
				ipResolver.GetPrimaryIPNet = &gonet.IPNet{
					IP:   gonet.ParseIP("127.0.0.1"),
					Mask: gonet.CIDRMask(16, 32),
				}

				_, err := interfaceAddress.GetIP(IPv4)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not attempt to resolve IP again", func() {
				ipResolver.GetPrimaryIPInterfaceName = ""

				ip, err := interfaceAddress.GetIP(IPv4)
				Expect(err).ToNot(HaveOccurred())
				Expect(ip).To(Equal("127.0.0.1"))

				Expect(ipResolver.GetPrimaryIPInterfaceName).To(Equal(""))
			})
		})

		Context("when GetIP was called with the right ipProtocol", func() {

			BeforeEach(func() {
				ipResolver.GetPrimaryIPNet = &gonet.IPNet{
					IP:   gonet.ParseIP("127.0.0.1"),
					Mask: gonet.CIDRMask(16, 32),
				}
			})

			for _, ipProtocol := range []IPProtocol{IPv4, IPv6} {
				var ipProtocolStr string
				switch ipProtocol {
				case IPv4:
					ipProtocolStr = "IPv4"
				case IPv6:
					ipProtocolStr = "IPv6"
				}
				It("it should have been called ipResolver with same value: "+ipProtocolStr, func() {
					_, err := interfaceAddress.GetIP(ipProtocol)
					Expect(err).ToNot(HaveOccurred())
					Expect(ipResolver.GetPrimaryIPCalledWith.IFaceName).To(Equal("fake-iface-name"))
					Expect(ipResolver.GetPrimaryIPCalledWith.IpProtocol).To(Equal(ipProtocol))
				})
			}

		})
	})
})
