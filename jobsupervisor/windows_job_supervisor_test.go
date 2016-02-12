package jobsupervisor_test

import (
	"runtime"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/jobsupervisor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WindowsJobSupervisor", func() {
	var (
		runner        boshsys.CmdRunner
		fs            *fakesys.FakeFileSystem
		jobSupervisor JobSupervisor
	)

	const (
		processConfigContents = `{
"processes": [
	{
		"name": "say-hello",
		"executable": "run.exe",
		"args": ["arg1"]
	}
]
}
`
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		runner = boshsys.NewExecCmdRunner(logger)
		jobSupervisor = NewWindowsJobSupervisor(runner, fs)
	})

	Context("when started", func() {
		BeforeEach(func() {
			jobSupervisor.Start()
		})

		It("reports running", func() {
			Expect(jobSupervisor.Status()).To(Equal("running"))
		})
	})

	Context("when stopped", func() {
		BeforeEach(func() {
			jobSupervisor.Stop()
		})

		It("reports stopped", func() {
			Expect(jobSupervisor.Status()).To(Equal("stopped"))
		})
	})

	Context("when unmonitored", func() {
		BeforeEach(func() {
			jobSupervisor.Unmonitor()
		})

		It("reports unmonitored", func() {
			Expect(jobSupervisor.Status()).To(Equal("unmonitored"))
		})
	})

	Describe("AddJob", func() {
		BeforeEach(func() {
			if runtime.GOOS != "windows" {
				Skip("Pending on non-Windows")
			}
		})

		It("creates a service with vcap description", func() {
			defer jobSupervisor.RemoveAllJobs()

			tempfile := fakesys.NewFakeFile("/fake-path", fs)
			fs.ReturnTempFile = tempfile

			fs.WriteFileString("/fake-process-config-path", processConfigContents)
			err := jobSupervisor.AddJob("fakeJob", 0, "/fake-process-config-path")
			Expect(err).ToNot(HaveOccurred())

			stdout, _, _, err := runner.RunCommand("powershell", "/C", "get-service", "say-hello")
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("say-hello"))
			Expect(stdout).To(ContainSubstring("Stopped"))
		})
	})
})
