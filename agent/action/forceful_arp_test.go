package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/platform/net/arp/fakes"
)

func init() {
	Describe("Forceful ARP", func() {
		var (
			arp       *fakes.FakeManager
			action    ForcefulARPAction
			addresses []string
		)

		BeforeEach(func() {
			arp = new(fakes.FakeManager)
			action = NewForcefulARP(arp)
			addresses = []string{"10.0.0.1", "10.0.0.2"}
		})

		It("is synchronous", func() {
			Expect(action.IsAsynchronous()).To(BeFalse())
		})

		It("is not persistent", func() {
			Expect(action.IsPersistent()).To(BeFalse())
		})

		It("requests deletion of all provided IPs from the ARP cache", func() {
			_, err := action.Run(addresses)

			Expect(err).ToNot(HaveOccurred())

			Expect(arp.DeleteCallCount()).To(Equal(len(addresses)))
			for i := 0; i < len(addresses); i++ {
				Expect(arp.DeleteArgsForCall(i)).To(Equal(addresses[i]))
			}
		})

		It("returns \"completed\"", func() {
			response, err := action.Run(addresses)

			Expect(err).ToNot(HaveOccurred())
			Expect(response).To(Equal("completed"))
		})
	})
}
