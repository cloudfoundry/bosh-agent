// +build windows

package jobsupervisor_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	boshalert "github.com/cloudfoundry/bosh-agent/agent/alert"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	. "github.com/cloudfoundry/bosh-agent/jobsupervisor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const jobFailuresServerPort = 5000

func testWindowsConfigs(jobName string) (WindowsProcessConfig, bool) {
	m := map[string]WindowsProcessConfig{
		"say-hello": WindowsProcessConfig{
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
		},
		"flapping": WindowsProcessConfig{
			Processes: []WindowsProcess{
				{
					Name:       fmt.Sprintf("flapping-1-%d", time.Now().UnixNano()),
					Executable: "powershell",
					Args:       []string{"/C", "Write-Host \"Flapping\"; Start-Sleep 1; exit 2"},
				},
			},
		},
	}
	conf, ok := m[jobName]
	return conf, ok
}

var _ = Describe("WindowsJobSupervisor", func() {
	Context("add jobs and control services", func() {
		BeforeEach(func() {
			if runtime.GOOS != "windows" {
				Skip("Pending on non-Windows")
			}
		})

		var (
			fs                boshsys.FileSystem
			logger            boshlog.Logger
			basePath          string
			logDir            string
			exePath           string
			jobDir            string
			processConfigPath string
			jobSupervisor     JobSupervisor
			runner            boshsys.CmdRunner
			logOut            *bytes.Buffer
			logErr            *bytes.Buffer
		)

		BeforeEach(func() {
			const testExtPath = "testdata/job-service-wrapper"

			logOut = bytes.NewBufferString("")
			logErr = bytes.NewBufferString("")

			logger = boshlog.NewWriterLogger(boshlog.LevelDebug, logOut, logErr)
			fs = boshsys.NewOsFileSystem(logger)

			var err error
			basePath, err = ioutil.TempDir("", "")
			Expect(err).ToNot(HaveOccurred())
			fs.MkdirAll(basePath, 0755)

			binPath := filepath.Join(basePath, "bosh", "bin")
			fs.MkdirAll(binPath, 0755)

			logDir = path.Join(basePath, "sys", "log")
			fs.MkdirAll(binPath, 0755)

			exePath = filepath.Join(binPath, "job-service-wrapper.exe")

			err = fs.CopyFile(testExtPath, exePath)
			Expect(err).ToNot(HaveOccurred())

			logDir = path.Join(basePath, "sys", "log")
		})

		WriteJobConfig := func(configContents WindowsProcessConfig) (string, error) {
			dirProvider := boshdirs.NewProvider(basePath)
			runner = boshsys.NewExecCmdRunner(logger)
			jobSupervisor = NewWindowsJobSupervisor(runner, dirProvider, fs, logger, jobFailuresServerPort, make(chan bool))
			if err := jobSupervisor.RemoveAllJobs(); err != nil {
				return "", err
			}
			processConfigContents, err := json.Marshal(configContents)
			if err != nil {
				return "", err
			}

			jobDir, err = fs.TempDir("testWindowsJobSupervisor")
			processConfigPath = filepath.Join(jobDir, "monit")

			err = fs.WriteFile(processConfigPath, processConfigContents)
			return processConfigPath, err
		}

		AddJob := func(jobName string) (WindowsProcessConfig, error) {
			conf, ok := testWindowsConfigs(jobName)
			if !ok {
				return conf, fmt.Errorf("Invalid Windows Config Process name: %s", jobName)
			}
			confPath, err := WriteJobConfig(conf)
			if err != nil {
				return conf, err
			}
			return conf, jobSupervisor.AddJob(jobName, 0, confPath)
		}

		AfterEach(func() {
			Expect(jobSupervisor.Stop()).To(Succeed())
			Expect(jobSupervisor.RemoveAllJobs()).To(Succeed())
			Expect(fs.RemoveAll(jobDir)).To(Succeed())
			Expect(fs.RemoveAll(logDir)).To(Succeed())
		})

		Describe("AddJob", func() {
			It("creates a service with vcap description", func() {
				conf, err := AddJob("say-hello")
				Expect(err).ToNot(HaveOccurred())

				for _, proc := range conf.Processes {
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
					_, err := AddJob("say-hello")
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Describe("Start", func() {
			var conf WindowsProcessConfig
			BeforeEach(func() {
				var err error
				conf, err = AddJob("say-hello")
				Expect(err).ToNot(HaveOccurred())
			})

			It("will start all the services", func() {
				Expect(jobSupervisor.Start()).To(Succeed())

				for _, proc := range conf.Processes {
					stdout, _, _, err := runner.RunCommand("powershell", "/C", "get-service", proc.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(stdout).To(ContainSubstring(proc.Name))
					Expect(stdout).To(ContainSubstring("Running"))
				}
			})

			It("writes logs to job log directory", func() {
				Expect(jobSupervisor.Start()).To(Succeed())

				for i, proc := range conf.Processes {
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
					_, err := AddJob("say-hello")
					Expect(err).ToNot(HaveOccurred())
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
			var conf WindowsProcessConfig
			BeforeEach(func() {
				var err error
				conf, err = AddJob("say-hello")
				Expect(err).ToNot(HaveOccurred())
			})

			It("sets service status to Disabled", func() {
				Expect(jobSupervisor.Unmonitor()).To(Succeed())

				for _, proc := range conf.Processes {
					stdout, _, _, err := runner.RunCommand(
						"powershell", "/C", "get-wmiobject", "win32_service", "-filter",
						fmt.Sprintf(`"name='%s'"`, proc.Name), "-property", "StartMode",
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(stdout).To(ContainSubstring("Disabled"))
				}
			})
		})

		GetServiceState := func(serviceName string) (svc.State, error) {
			m, err := mgr.Connect()
			if err != nil {
				return 0, err
			}
			defer m.Disconnect()
			s, err := m.OpenService(serviceName)
			if err != nil {
				return 0, err
			}
			defer s.Close()
			st, err := s.Query()
			if err != nil {
				return 0, err
			}
			return st.State, nil
		}

		StateString := func(s svc.State) string {
			switch s {
			case svc.Stopped:
				return "Stopped"
			case svc.StartPending:
				return "StartPending"
			case svc.StopPending:
				return "StopPending"
			case svc.Running:
				return "Running"
			case svc.ContinuePending:
				return "ContinuePending"
			case svc.PausePending:
				return "PausePending"
			case svc.Paused:
				return "Paused"
			}
			return fmt.Sprintf("Unkown State: %d", s)
		}
		var _ = StateString

		Describe("Stop", func() {
			It("sets service status to Stopped", func() {
				conf, err := AddJob("say-hello")
				Expect(err).ToNot(HaveOccurred())

				Expect(jobSupervisor.Start()).To(Succeed())
				Expect(jobSupervisor.Stop()).To(Succeed())

				for _, proc := range conf.Processes {
					Eventually(func() (string, error) {
						st, err := GetServiceState(proc.Name)
						return StateString(st), err
					}).Should(Equal(StateString(svc.Stopped)))
				}
			})

			It("can start a stopped service", func() {
				conf, err := AddJob("say-hello")
				Expect(err).ToNot(HaveOccurred())

				Expect(jobSupervisor.Start()).To(Succeed())
				Expect(jobSupervisor.Stop()).To(Succeed())

				for _, proc := range conf.Processes {
					Eventually(func() (string, error) {
						st, err := GetServiceState(proc.Name)
						return StateString(st), err
					}).Should(Equal(StateString(svc.Stopped)))
				}

				Expect(jobSupervisor.Start()).To(Succeed())
				for _, proc := range conf.Processes {
					Eventually(func() (string, error) {
						st, err := GetServiceState(proc.Name)
						return StateString(st), err
					}).Should(Equal(StateString(svc.Running)))
				}
			})

			It("stops flapping services", func() {
				conf, err := AddJob("flapping")
				Expect(err).ToNot(HaveOccurred())
				Expect(jobSupervisor.Start()).To(Succeed())

				Expect(len(conf.Processes)).To(Equal(1))
				proc := conf.Processes[0]
				Eventually(func() (string, error) {
					st, err := GetServiceState(proc.Name)
					return StateString(st), err
				}, time.Second*6).Should(Equal(StateString(svc.Stopped)))

				Expect(jobSupervisor.Stop()).To(Succeed())

				Eventually(func() (string, error) {
					st, err := GetServiceState(proc.Name)
					return StateString(st), err
				}, time.Second*6).Should(Equal(StateString(svc.Stopped)))

				Consistently(func() (string, error) {
					st, err := GetServiceState(proc.Name)
					return StateString(st), err
				}, time.Second*6, time.Millisecond*10).Should(Equal(StateString(svc.Stopped)))

			})
		})

		Describe("MonitorJobFailures", func() {
			var cancelServer chan bool
			BeforeEach(func() {
				dirProvider := boshdirs.NewProvider(basePath)
				runner = boshsys.NewExecCmdRunner(logger)
				cancelServer = make(chan bool)
				jobSupervisor = NewWindowsJobSupervisor(runner, dirProvider, fs, logger, jobFailuresServerPort, cancelServer)
			})

			AfterEach(func() {
				cancelServer <- true
			})

			doJobFailureRequest := func(payload string, port int) error {
				url := fmt.Sprintf("http://localhost:%d", port)
				buf := bytes.NewBufferString(payload)
				_, err := http.Post(url, "application/json", buf)
				return err
			}

			It("receives job failures from the service wrapper via HTTP", func() {
				var handledAlert boshalert.MonitAlert

				failureHandler := func(alert boshalert.MonitAlert) (err error) {
					handledAlert = alert
					return
				}

				go jobSupervisor.MonitorJobFailures(failureHandler)

				msg := `{"datetime":"Sun, 22 May 2011 20:07:41 +0500",
								 "event": "pid failed",
								 "processName": "nats",
								 "exitCode":55}`

				err := doJobFailureRequest(msg, jobFailuresServerPort)
				Expect(err).ToNot(HaveOccurred())

				Expect(handledAlert).To(Equal(boshalert.MonitAlert{
					ID:          "nats",
					Service:     "nats",
					Event:       "pid failed",
					Action:      "Start",
					Date:        "Sun, 22 May 2011 20:07:41 +0500",
					Description: "exited with code 55",
				}))
			})

			It("ignores unknown requests", func() {
				var didHandleAlert bool

				failureHandler := func(alert boshalert.MonitAlert) (err error) {
					didHandleAlert = true
					return
				}

				go jobSupervisor.MonitorJobFailures(failureHandler)

				err := doJobFailureRequest(`some bad request`, jobFailuresServerPort)
				Expect(err).ToNot(HaveOccurred())
				Expect(didHandleAlert).To(BeFalse())
				Expect(logErr.Bytes()).To(ContainSubstring("MonitorJobFailures received unknown request"))
			})

			It("returns an error when it fails to bind", func() {
				failureHandler := func(alert boshalert.MonitAlert) (err error) { return }

				go jobSupervisor.MonitorJobFailures(failureHandler)
				time.Sleep(50 * time.Millisecond)
				err := jobSupervisor.MonitorJobFailures(failureHandler)
				Expect(err).To(HaveOccurred())
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
