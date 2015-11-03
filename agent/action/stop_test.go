package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor/fakes"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

func init() {
	Describe("Stop", func() {
		var (
			jobSupervisor *fakejobsuper.FakeJobSupervisor
			fs            *fakesys.FakeFileSystem
			dirProvider   boshdir.Provider
			action        StopAction
		)

		BeforeEach(func() {
			jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
			fs = fakesys.NewFakeFileSystem()
			dirProvider = boshdir.NewProvider("/var/vcap")
			action = NewStop(jobSupervisor, fs, dirProvider)
		})

		It("is asynchronous", func() {
			Expect(action.IsAsynchronous()).To(BeTrue())
		})

		It("is not persistent", func() {
			Expect(action.IsPersistent()).To(BeFalse())
		})

		It("returns stopped", func() {
			stopped, err := action.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(stopped).To(Equal("stopped"))
		})

		It("stops job supervisor services", func() {
			_, err := action.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(jobSupervisor.Stopped).To(BeTrue())
		})

		It("creates stopped file", func() {
			_, err := action.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(fs.FileExists("/var/vcap/monit/stopped")).To(BeTrue())
		})
	})
}
