package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
)

var _ = Describe("Shutdown", func() {
	var (
		platform       *platformfakes.FakePlatform
		shutdownAction action.ShutdownAction
	)

	BeforeEach(func() {
		platform = &platformfakes.FakePlatform{}
		shutdownAction = action.NewShutdown(platform)
	})

	AssertActionIsNotAsynchronous(shutdownAction)
	AssertActionIsNotPersistent(shutdownAction)
	AssertActionIsLoggable(shutdownAction)

	AssertActionIsNotCancelable(shutdownAction)
	AssertActionIsNotResumable(shutdownAction)

	It("shuts the VM down", func() {
		_, err := shutdownAction.Run()
		Expect(err).ToNot(HaveOccurred())
		Expect(platform.ShutdownCallCount()).To(Equal(1))
	})

	It("returns an empty string", func() {
		response, err := shutdownAction.Run()
		Expect(err).ToNot(HaveOccurred())
		Expect(response).To(Equal(""))
	})
})
