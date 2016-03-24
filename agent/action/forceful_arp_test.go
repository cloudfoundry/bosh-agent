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
			args      ForcefulARPActionArgs
		)

		BeforeEach(func() {
			arp = new(fakes.FakeManager)
			action = NewForcefulARP(arp)
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
			_, err := action.Run(args)

			Expect(err).ToNot(HaveOccurred())

			Expect(arp.DeleteCallCount()).To(Equal(len(addresses)))
			for i, address := range addresses {
				Expect(arp.DeleteArgsForCall(i)).To(Equal(address))
			}
		})

		It("returns an empty map", func() {
			response, err := action.Run(args)

			Expect(err).ToNot(HaveOccurred())
			Expect(response).To(Equal(map[string]interface{}{}))
		})
	})
}
