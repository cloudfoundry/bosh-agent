package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	fakeas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec/fakes"
	fakeappl "github.com/cloudfoundry/bosh-agent/agent/applier/fakes"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor/fakes"
)

var _ = Describe("Start", func() {
	var (
		jobSupervisor *fakejobsuper.FakeJobSupervisor
		applier       *fakeappl.FakeApplier
		specService   *fakeas.FakeV1Service
		startAction   action.StartAction
	)

	BeforeEach(func() {
		jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
		applier = fakeappl.NewFakeApplier()
		specService = fakeas.NewFakeV1Service()
		startAction = action.NewStart(jobSupervisor, applier, specService)
	})

	AssertActionIsNotAsynchronous(startAction)
	AssertActionIsNotPersistent(startAction)
	AssertActionIsLoggable(startAction)

	AssertActionIsNotResumable(startAction)
	AssertActionIsNotCancelable(startAction)

	It("returns started", func() {
		started, err := startAction.Run()
		Expect(err).ToNot(HaveOccurred())
		Expect(started).To(Equal("started"))
	})

	It("starts monitor services", func() {
		_, err := startAction.Run()
		Expect(err).ToNot(HaveOccurred())
		Expect(jobSupervisor.Started).To(BeTrue())
	})

	It("configures jobs", func() {
		_, err := startAction.Run()
		Expect(err).ToNot(HaveOccurred())
		Expect(applier.Configured).To(BeTrue())
	})

	It("apply errs if a job fails configuring", func() {
		applier.ConfiguredError = errors.New("fake error")
		_, err := startAction.Run()

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Configuring jobs"))
	})
})
