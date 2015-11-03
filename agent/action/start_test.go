package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	. "github.com/cloudfoundry/bosh-agent/agent/action"
	fakeas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec/fakes"
	fakeappl "github.com/cloudfoundry/bosh-agent/agent/applier/fakes"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor/fakes"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	"os"
)

func init() {
	Describe("Start", func() {
		var (
			jobSupervisor *fakejobsuper.FakeJobSupervisor
			applier       *fakeappl.FakeApplier
			specService   *fakeas.FakeV1Service
			fs            *fakesys.FakeFileSystem
			dirProvider   boshdir.Provider
			action        StartAction
		)

		BeforeEach(func() {
			jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
			applier = fakeappl.NewFakeApplier()
			specService = fakeas.NewFakeV1Service()
			fs = fakesys.NewFakeFileSystem()
			dirProvider = boshdir.NewProvider("/var/vcap")
			action = NewStart(jobSupervisor, applier, specService, fs, dirProvider)
		})

		It("is synchronous", func() {
			Expect(action.IsAsynchronous()).To(BeFalse())
		})

		It("is not persistent", func() {
			Expect(action.IsPersistent()).To(BeFalse())
		})

		It("returns started", func() {
			started, err := action.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(started).To(Equal("started"))
		})

		It("starts monitor services", func() {
			_, err := action.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(jobSupervisor.Started).To(BeTrue())
		})

		It("configures jobs", func() {
			_, err := action.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(applier.Configured).To(BeTrue())
		})

		It("deletes stopped file", func() {
			fs.MkdirAll("/var/vcap/monit/stopped", os.FileMode(0755))
			fs.WriteFileString("/var/vcap/monit/stopped", "")

			_, err := action.Run()
			Expect(err).ToNot(HaveOccurred())

			Expect(fs.FileExists("/var/vcap/monit/stopped")).ToNot(BeTrue())
		})

		It("apply errs if a job fails configuring", func() {
			applier.ConfiguredError = errors.New("fake error")
			_, err := action.Run()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Configuring jobs"))
		})
	})
}
