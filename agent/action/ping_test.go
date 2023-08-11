package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
)

var _ = Describe("Ping", func() {

	var (
		pingAction action.PingAction
	)

	BeforeEach(func() {
		pingAction = action.NewPing()
	})

	AssertActionIsNotAsynchronous(pingAction)
	AssertActionIsNotPersistent(pingAction)
	AssertActionIsLoggable(pingAction)

	AssertActionIsNotResumable(pingAction)
	AssertActionIsNotCancelable(pingAction)

	It("ping run returns pong", func() {
		pong, err := pingAction.Run()
		Expect(err).ToNot(HaveOccurred())
		Expect(pong).To(Equal("pong"))
	})
})
