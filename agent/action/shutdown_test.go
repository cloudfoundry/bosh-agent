package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
)

var _ = Describe("Shutdown", func() {
	var (
		platform *fakeplatform.FakePlatform
		action   ShutdownAction
	)

	BeforeEach(func() {
		platform = new(fakeplatform.FakePlatform)
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
		Expect(platform.ShutdownCalled).To(BeTrue())
	})

	It("returns an empty string", func() {
		response, err := action.Run()

		Expect(err).ToNot(HaveOccurred())
		Expect(response).To(Equal(""))
	})
})
