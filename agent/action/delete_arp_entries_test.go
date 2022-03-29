package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
)

var _ = Describe("Delete ARP Entries", func() {
	var (
		platform               *platformfakes.FakePlatform
		deleteARPEntriesAction action.DeleteARPEntriesAction
		addresses              []string
		args                   action.DeleteARPEntriesActionArgs
	)

	BeforeEach(func() {
		platform = &platformfakes.FakePlatform{}
		deleteARPEntriesAction = action.NewDeleteARPEntries(platform)
		addresses = []string{"10.0.0.1", "10.0.0.2"}
		args = action.DeleteARPEntriesActionArgs{
			Ips: addresses,
		}
	})

	AssertActionIsNotAsynchronous(deleteARPEntriesAction)
	AssertActionIsNotPersistent(deleteARPEntriesAction)
	AssertActionIsLoggable(deleteARPEntriesAction)

	AssertActionIsNotCancelable(deleteARPEntriesAction)
	AssertActionIsNotResumable(deleteARPEntriesAction)

	It("requests deletion of all provided IPs from the ARP cache", func() {
		_, err := deleteARPEntriesAction.Run(args)
		Expect(err).ToNot(HaveOccurred())
		Expect(platform.DeleteARPEntryWithIPCallCount()).To(Equal(2))
		Expect(platform.DeleteARPEntryWithIPArgsForCall(0)).To(Equal("10.0.0.1"))
		Expect(platform.DeleteARPEntryWithIPArgsForCall(1)).To(Equal("10.0.0.2"))
	})

	It("returns an empty map", func() {
		response, err := deleteARPEntriesAction.Run(args)

		Expect(err).ToNot(HaveOccurred())
		Expect(response).To(Equal(map[string]interface{}{}))
	})
})
