package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
)

func init() {
	Describe("Forceful ARP", func() {
		var (
			platform  *fakeplatform.FakePlatform
			action    ForcefulARPAction
			addresses []string
			args      ForcefulARPActionArgs
		)

		BeforeEach(func() {
			platform = new(fakeplatform.FakePlatform)
			action = NewForcefulARP(platform)
			addresses = []string{"10.0.0.1", "10.0.0.2"}
			args = ForcefulARPActionArgs{
				Ips: addresses,
			}
		})

		It("is asynchronous", func() {
			Expect(action.IsAsynchronous()).To(BeTrue())
		})

		It("is not persistent", func() {
			Expect(action.IsPersistent()).To(BeFalse())
		})

		It("requests deletion of all provided IPs from the ARP cache", func() {
			Expect(platform.CleanedIPMacAddressCache).To(Equal(""))
			_, err := action.Run(args)
			Expect(err).ToNot(HaveOccurred())
			Expect(platform.CleanedIPMacAddressCache).To(Equal("10.0.0.2"))
		})

		It("returns an empty map", func() {
			response, err := action.Run(args)

			Expect(err).ToNot(HaveOccurred())
			Expect(response).To(Equal(map[string]interface{}{}))
		})
	})
}
