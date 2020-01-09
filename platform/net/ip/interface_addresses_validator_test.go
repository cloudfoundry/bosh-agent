package ip_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boship "github.com/cloudfoundry/bosh-agent/platform/net/ip"
	fakeip "github.com/cloudfoundry/bosh-agent/platform/net/ip/fakes"
)

var _ = Describe("InterfaceAddressesValidator", func() {
	var (
		interfaceAddrsProvider  *fakeip.FakeInterfaceAddressesProvider
		interfaceAddrsValidator boship.InterfaceAddressesValidator
	)

	BeforeEach(func() {
		interfaceAddrsProvider = &fakeip.FakeInterfaceAddressesProvider{}
	})

	Context("when networks match", func() {
		BeforeEach(func() {
			interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "1.2.3.4"),
				boship.NewSimpleInterfaceAddress("eth1", "5.6.7.8"),
			}
		})

		It("returns nil", func() {
			interfaceAddrsValidator = boship.NewInterfaceAddressesValidator(interfaceAddrsProvider, []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "1.2.3.4"),
			})
			retry, err := interfaceAddrsValidator.Attempt()
			Expect(retry).To(Equal(false))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when an interface has multiple IPs", func() {
		BeforeEach(func() {
			interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "1.2.3.4"),
				boship.NewSimpleInterfaceAddress("eth0", "fe80::1"),
			}
		})

		It("returns nil", func() {
			interfaceAddrsValidator = boship.NewInterfaceAddressesValidator(interfaceAddrsProvider, []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "fe80::1"),
			})
			retry, err := interfaceAddrsValidator.Attempt()
			Expect(retry).To(Equal(false))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when desired networks do not match actual network IP address", func() {
		BeforeEach(func() {
			interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "1.2.3.5"),
				boship.NewSimpleInterfaceAddress("eth0", "1.2.3.6"),
			}
		})

		It("fails", func() {
			interfaceAddrsValidator = boship.NewInterfaceAddressesValidator(interfaceAddrsProvider, []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "1.2.3.4"),
			})
			retry, err := interfaceAddrsValidator.Attempt()
			Expect(retry).To(Equal(true))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Validating network interface 'eth0' IP addresses, expected: '1.2.3.4', actual: [1.2.3.5, 1.2.3.6]"))
		})
	})

	Context("when validating manual networks fails", func() {
		BeforeEach(func() {
			interfaceAddrsProvider.GetErr = errors.New("interface-error")
		})

		It("fails", func() {
			interfaceAddrsValidator = boship.NewInterfaceAddressesValidator(interfaceAddrsProvider, []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "1.2.3.4"),
			})
			retry, err := interfaceAddrsValidator.Attempt()
			Expect(retry).To(Equal(true))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("interface-error"))
		})
	})

	Context("when interface is not configured", func() {
		BeforeEach(func() {
			interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("another-ethstatic", "1.2.3.5"),
			}
		})

		It("fails", func() {
			interfaceAddrsValidator = boship.NewInterfaceAddressesValidator(interfaceAddrsProvider, []boship.InterfaceAddress{
				boship.NewSimpleInterfaceAddress("eth0", "1.2.3.4"),
			})
			retry, err := interfaceAddrsValidator.Attempt()
			Expect(retry).To(Equal(true))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Validating network interface 'eth0' IP addresses, no interface configured with that name"))
		})
	})
})
