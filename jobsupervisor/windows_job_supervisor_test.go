package jobsupervisor_test

import (
	"encoding/json"
	"path"
	"path/filepath"
	"runtime"
	"time"

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
		runner            boshsys.CmdRunner
		fs                boshsys.FileSystem
		jobSupervisor     JobSupervisor
		jobDir            string
		processConfigPath string
		basePath          string
		logDir            string
	)

	AddJob := func() error {
		return jobSupervisor.AddJob("say-hello", 0, processConfigPath)
	}

	BeforeEach(func() {
		basePath = "C:/var/vcap/"
		logDir = path.Join(basePath, "sys", "log")

		configContents := WindowsProcessConfig{
			Processes: []WindowsProcess{
				{
					Name:       "say-hello",
					Executable: "powershell",
					Args:       []string{"/C", "Write-Host \"Hello\"; Start-Sleep 10"},
				},
			},
		}

		processConfigContents, err := json.Marshal(configContents)
		Expect(err).ToNot(HaveOccurred())

		logger := boshlog.NewLogger(boshlog.LevelNone)
		dirProvider := boshdirs.NewProvider(basePath)

		fs = boshsys.NewOsFileSystem(logger)
		runner = boshsys.NewExecCmdRunner(logger)
		jobSupervisor = NewWindowsJobSupervisor(runner, dirProvider, fs, logger)
		jobSupervisor.RemoveAllJobs()

		jobDir, err = fs.TempDir("testWindowsJobSupervisor")
		processConfigPath = filepath.Join(jobDir, "monit")

		err = fs.WriteFile(processConfigPath, processConfigContents)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		jobSupervisor.Stop()
		jobSupervisor.RemoveAllJobs()
		fs.RemoveAll(jobDir)
		fs.RemoveAll(logDir)
	})

	Describe("AddJob", func() {
		It("creates a service with vcap description", func() {
			Expect(AddJob()).ToNot(HaveOccurred())

			stdout, _, _, err := runner.RunCommand("powershell", "/C", "get-service", "say-hello")
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("say-hello"))
			Expect(stdout).To(ContainSubstring("Stopped"))
		})

		Context("when monit file is empty", func() {
			BeforeEach(func() {
				err := fs.WriteFileString(processConfigPath, "")
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not return an error", func() {
				Expect(AddJob()).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Start", func() {
		BeforeEach(func() {
			Expect(AddJob()).ToNot(HaveOccurred())
		})

		It("will start all the services", func() {
			err := jobSupervisor.Start()
			Expect(err).ToNot(HaveOccurred())

			stdout, _, _, err := runner.RunCommand("powershell", "/C", "get-service", "say-hello")
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("say-hello"))
			Expect(stdout).To(ContainSubstring("Running"))
		})

		It("writes logs to job log directory", func() {
			err := jobSupervisor.Start()
			Expect(err).ToNot(HaveOccurred())

			readLogFile := func() (string, error) {
				return fs.ReadFileString(path.Join(logDir, "say-hello", "say-hello", "job-service-wrapper.out.log"))
			}

			Eventually(readLogFile, 10*time.Second, 500*time.Millisecond).Should(ContainSubstring("Hello"))
		})
	})

	Describe("Status", func() {
		Context("with jobs", func() {
			BeforeEach(func() {
				Expect(AddJob()).ToNot(HaveOccurred())
			})

			Context("when running", func() {
				It("reports that the job is 'Running'", func() {
					err := jobSupervisor.Start()
					Expect(err).ToNot(HaveOccurred())

					Expect(jobSupervisor.Status()).To(Equal("running"))
				})
			})

			Context("when stopped", func() {
				It("reports that the job is 'Stopped'", func() {
					err := jobSupervisor.Start()
					Expect(err).ToNot(HaveOccurred())

					err = jobSupervisor.Stop()
					Expect(err).ToNot(HaveOccurred())

					Expect(jobSupervisor.Status()).To(Equal("stopped"))
				})
			})
		})

		Context("with no jobs", func() {
			Context("when running", func() {
				It("reports that the job is 'Running'", func() {
					err := jobSupervisor.Start()
					Expect(err).ToNot(HaveOccurred())

					Expect(jobSupervisor.Status()).To(Equal("running"))
				})
			})
		})
	})
})
