package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
)

var _ = Describe("Delete ARP Entries", func() {
	var (
		platform  *platformfakes.FakePlatform
		action    DeleteARPEntriesAction
		addresses []string
		args      DeleteARPEntriesActionArgs
	)

	BeforeEach(func() {
		platform = &platformfakes.FakePlatform{}
		action = NewDeleteARPEntries(platform)
		addresses = []string{"10.0.0.1", "10.0.0.2"}
		args = DeleteARPEntriesActionArgs{
			Ips: addresses,
		}
	})

	AssertActionIsNotAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotCancelable(action)
	AssertActionIsNotResumable(action)

	It("requests deletion of all provided IPs from the ARP cache", func() {
		_, err := action.Run(args)
		Expect(err).ToNot(HaveOccurred())
		Expect(platform.DeleteARPEntryWithIPCallCount()).To(Equal(2))
		Expect(platform.DeleteARPEntryWithIPArgsForCall(0)).To(Equal("10.0.0.1"))
		Expect(platform.DeleteARPEntryWithIPArgsForCall(1)).To(Equal("10.0.0.2"))
	})

	It("returns an empty map", func() {
		response, err := action.Run(args)

		Expect(err).ToNot(HaveOccurred())
		Expect(response).To(Equal(map[string]interface{}{}))
	})
})
