package jobsupervisor_test

import (
	"path/filepath"
	"runtime"

	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	. "github.com/cloudfoundry/bosh-agent/jobsupervisor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WindowsJobSupervisor", func() {
	BeforeEach(func() {
		if runtime.GOOS != "windows" {
			Skip("Pending on non-Windows")
		}
	})

	var (
		runner        boshsys.CmdRunner
		fs            boshsys.FileSystem
		jobSupervisor JobSupervisor
	)

	const (
		processConfigContents = `{
"processes": [
	{
		"name": "say-hello",
		"executable": "powershell",
		"args": ["/C", "Start-Sleep 10"]
	}
]
}
`
	)

	BeforeEach(func() {
		logger := boshlog.NewLogger(boshlog.LevelDebug)
		dirProvider := boshdirs.NewProvider("/var/vcap/")

		fs = boshsys.NewOsFileSystem(logger)
		runner = boshsys.NewExecCmdRunner(logger)
		jobSupervisor = NewWindowsJobSupervisor(runner, dirProvider, fs, logger)
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

	Context("with service", func() {
		var (
			jobDir string
		)

		BeforeEach(func() {
			var err error
			jobDir, err = fs.TempDir("testWindowsJobSupervisor")
			processConfigPath := filepath.Join(jobDir, "monit")

			err = fs.WriteFileString(processConfigPath, processConfigContents)
			Expect(err).ToNot(HaveOccurred())

			err = jobSupervisor.AddJob("say-hello", 0, processConfigPath)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			jobSupervisor.Stop()
			jobSupervisor.RemoveAllJobs()
			fs.RemoveAll(jobDir)
		})

		Describe("AddJob", func() {
			It("creates a service with vcap description", func() {
				stdout, _, _, err := runner.RunCommand("powershell", "/C", "get-service", "say-hello")
				Expect(err).ToNot(HaveOccurred())
				Expect(stdout).To(ContainSubstring("say-hello"))
				Expect(stdout).To(ContainSubstring("Stopped"))
			})
		})

		Describe("Start", func() {
			It("will start all the services", func() {
				jobSupervisor.Start()

				stdout, _, _, err := runner.RunCommand("powershell", "/C", "get-service", "say-hello")
				Expect(err).ToNot(HaveOccurred())
				Expect(stdout).To(ContainSubstring("say-hello"))
				Expect(stdout).To(ContainSubstring("Running"))
			})
		})
	})
})
