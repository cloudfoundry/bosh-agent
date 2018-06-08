package action_test

import (
	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
)

var _ = Describe("Shutdown", func() {
	var (
		platform *platformfakes.FakePlatform
		action   ShutdownAction
	)

	BeforeEach(func() {
		platform = &platformfakes.FakePlatform{}
		action = NewShutdown(platform)
	})

	AssertActionIsNotAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotCancelable(action)
	AssertActionIsNotResumable(action)

	It("shuts the VM down", func() {
		_, err := action.Run()
		Expect(err).ToNot(HaveOccurred())
		Expect(platform.ShutdownCallCount()).To(Equal(1))
	})

	It("returns an empty string", func() {
		response, err := action.Run()
		Expect(err).ToNot(HaveOccurred())
		Expect(response).To(Equal(""))
	})
})
