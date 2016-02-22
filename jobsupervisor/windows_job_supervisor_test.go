package jobsupervisor_test

import (
	"encoding/json"
	"fmt"
	"os"
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
	Context("add jobs and control services", func() {
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
			exePath           string
			logDir            string
			exePathNotExist   bool
			configContents    WindowsProcessConfig
		)

		AddJob := func() error {
			return jobSupervisor.AddJob("say-hello", 0, processConfigPath)
		}

		BeforeEach(func() {
			logger := boshlog.NewLogger(boshlog.LevelNone)
			fs = boshsys.NewOsFileSystem(logger)

			basePath = "C:/var/vcap/"
			fs.MkdirAll(basePath, 0755)

			binPath := filepath.Join(basePath, "bosh", "bin")
			fs.MkdirAll(binPath, 0755)

			logDir = path.Join(basePath, "sys", "log")
			fs.MkdirAll(binPath, 0755)

			const testExtPath = "testdata/job-service-wrapper"
			exePath = filepath.Join(binPath, "job-service-wrapper.exe")

			_, err := os.Stat(exePath)
			exePathNotExist = os.IsNotExist(err)
			if exePathNotExist {
				Expect(fs.CopyFile(testExtPath, exePath)).To(Succeed())
			}

			logDir = path.Join(basePath, "sys", "log")

			configContents = WindowsProcessConfig{
				Processes: []WindowsProcess{
					{
						Name:       fmt.Sprintf("say-hello-1-%d", time.Now().UnixNano()),
						Executable: "powershell",
						Args:       []string{"/C", "Write-Host \"Hello 1\"; Start-Sleep 10"},
					},
					{
						Name:       fmt.Sprintf("say-hello-2-%d", time.Now().UnixNano()),
						Executable: "powershell",
						Args:       []string{"/C", "Write-Host \"Hello 2\"; Start-Sleep 10"},
					},
				},
			}

			processConfigContents, err := json.Marshal(configContents)
			Expect(err).ToNot(HaveOccurred())

			dirProvider := boshdirs.NewProvider(basePath)

			runner = boshsys.NewExecCmdRunner(logger)
			jobSupervisor = NewWindowsJobSupervisor(runner, dirProvider, fs, logger)
			Expect(jobSupervisor.RemoveAllJobs()).To(Succeed())

			jobDir, err = fs.TempDir("testWindowsJobSupervisor")
			processConfigPath = filepath.Join(jobDir, "monit")

			Expect(fs.WriteFile(processConfigPath, processConfigContents)).To(Succeed())
		})

		AfterEach(func() {
			Expect(jobSupervisor.Stop()).To(Succeed())
			Expect(jobSupervisor.RemoveAllJobs()).To(Succeed())
			Expect(fs.RemoveAll(jobDir)).To(Succeed())
			Expect(fs.RemoveAll(logDir)).To(Succeed())
			if exePathNotExist {
				Expect(fs.RemoveAll(exePath)).To(Succeed())
			}
		})

		Describe("AddJob", func() {
			It("creates a service with vcap description", func() {
				Expect(AddJob()).To(Succeed())

				for _, proc := range configContents.Processes {
					stdout, _, _, err := runner.RunCommand("powershell", "/C", "get-service", proc.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(stdout).To(ContainSubstring(proc.Name))
					Expect(stdout).To(ContainSubstring("Stopped"))
				}
			})

			Context("when monit file is empty", func() {
				BeforeEach(func() {
					Expect(fs.WriteFileString(processConfigPath, "")).To(Succeed())
				})

				It("does not return an error", func() {
					Expect(AddJob()).To(Succeed())
				})
			})
		})

		Describe("Start", func() {
			BeforeEach(func() {
				Expect(AddJob()).To(Succeed())
			})

			It("will start all the services", func() {
				Expect(jobSupervisor.Start()).To(Succeed())

				for _, proc := range configContents.Processes {
					stdout, _, _, err := runner.RunCommand("powershell", "/C", "get-service", proc.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(stdout).To(ContainSubstring(proc.Name))
					Expect(stdout).To(ContainSubstring("Running"))
				}
			})

			It("writes logs to job log directory", func() {
				Expect(jobSupervisor.Start()).To(Succeed())

				for i, proc := range configContents.Processes {
					readLogFile := func() (string, error) {
						return fs.ReadFileString(path.Join(logDir, "say-hello", proc.Name, "job-service-wrapper.out.log"))
					}

					Eventually(readLogFile, 10*time.Second, 500*time.Millisecond).Should(ContainSubstring(fmt.Sprintf("Hello %d", i+1)))
				}
			})
		})

		Describe("Status", func() {
			Context("with jobs", func() {
				BeforeEach(func() {
					Expect(AddJob()).To(Succeed())
				})

				Context("when running", func() {
					It("reports that the job is 'Running'", func() {
						Expect(jobSupervisor.Start()).To(Succeed())

						Expect(jobSupervisor.Status()).To(Equal("running"))
					})
				})

				Context("when stopped", func() {
					It("reports that the job is 'Stopped'", func() {
						Expect(jobSupervisor.Start()).To(Succeed())

						Expect(jobSupervisor.Stop()).To(Succeed())

						Expect(jobSupervisor.Status()).To(Equal("stopped"))
					})
				})
			})

			Context("with no jobs", func() {
				Context("when running", func() {
					It("reports that the job is 'Running'", func() {
						Expect(jobSupervisor.Start()).To(Succeed())

						Expect(jobSupervisor.Status()).To(Equal("running"))
					})
				})
			})
		})

		Describe("Unmonitor", func() {
			BeforeEach(func() {
				Expect(AddJob()).To(Succeed())
			})

			It("sets service status to Disabled", func() {
				Expect(jobSupervisor.Unmonitor()).To(Succeed())

				for _, proc := range configContents.Processes {
					stdout, _, _, err := runner.RunCommand(
						"powershell", "/C", "get-wmiobject", "win32_service", "-filter",
						fmt.Sprintf(`"name='%s'"`, proc.Name), "-property", "StartMode",
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(stdout).To(ContainSubstring("Disabled"))
				}
			})
		})
	})

	Describe("WindowsProcess#ServiceWrapperConfig", func() {
		Context("when the WindowsProcess has environment variables", func() {
			It("adds them to the marshalled WindowsServiceWrapperConfig XML", func() {
				proc := WindowsProcess{
					Name:       "Name",
					Executable: "Executable",
					Args:       []string{"A", "B"},
					Env: map[string]string{
						"Key_1": "Val_1",
						"Key_2": "Val_2",
					},
				}
				srvc := proc.ServiceWrapperConfig("LogPath")
				Expect(len(srvc.Env)).To(Equal(len(proc.Env)))
				for _, e := range srvc.Env {
					Expect(e.Value).To(Equal(proc.Env[e.Name]))
				}
			})
		})
	})
})
