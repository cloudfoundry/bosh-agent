package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/fakes"
)

var _ = Describe("Stop", func() {
	var (
		jobSupervisor *fakejobsuper.FakeJobSupervisor
		stopAction    action.StopAction
	)

	BeforeEach(func() {
		jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
		stopAction = action.NewStop(jobSupervisor)
	})

	AssertActionIsAsynchronous(stopAction)
	AssertActionIsNotPersistent(stopAction)
	AssertActionIsLoggable(stopAction)

	AssertActionIsNotResumable(stopAction)
	AssertActionIsNotCancelable(stopAction)

	It("returns stopped", func() {
		stopped, err := stopAction.Run(action.ProtocolVersion(2))
		Expect(err).ToNot(HaveOccurred())
		Expect(stopped).To(Equal("stopped"))
	})

	It("stops job supervisor services", func() {
		_, err := stopAction.Run(action.ProtocolVersion(2))
		Expect(err).ToNot(HaveOccurred())
		Expect(jobSupervisor.Stopped).To(BeTrue())
	})

	It("stops when protocol version is 2", func() {
		_, err := stopAction.Run(action.ProtocolVersion(2))
		Expect(err).ToNot(HaveOccurred())
		Expect(jobSupervisor.StoppedAndWaited).ToNot(BeTrue())
	})

	It("stops and waits when protocol version is greater than 2", func() {
		_, err := stopAction.Run(action.ProtocolVersion(3))
		Expect(err).ToNot(HaveOccurred())
		Expect(jobSupervisor.StoppedAndWaited).To(BeTrue())
	})
})
