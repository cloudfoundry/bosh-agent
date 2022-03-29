package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
)

var _ = Describe("Info", func() {
	var (
		infoAction action.InfoAction
	)

	BeforeEach(func() {
		infoAction = action.NewInfo()
	})

	AssertActionIsNotAsynchronous(infoAction)
	AssertActionIsNotPersistent(infoAction)
	AssertActionIsLoggable(infoAction)

	AssertActionIsNotResumable(infoAction)
	AssertActionIsNotCancelable(infoAction)

	It("returns the api version", func() {
		infoResponse, err := infoAction.Run()
		Expect(err).ToNot(HaveOccurred())
		Expect(infoResponse.APIVersion).To(Equal(1))
	})
})
