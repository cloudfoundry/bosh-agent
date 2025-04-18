package jobsupervisor_test

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	boshalert "github.com/cloudfoundry/bosh-agent/v2/agent/alert"
	. "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor"
	boshmonit "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/monit"
	fakemonit "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/monit/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/servicemanager/servicemanagerfakes"
	boshdir "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

var _ = Describe("monitJobSupervisor", func() {
	var (
		fs                    *fakesys.FakeFileSystem
		runner                *fakesys.FakeCmdRunner
		client                *fakemonit.FakeMonitClient
		logger                boshlog.Logger
		dirProvider           boshdir.Provider
		jobFailuresServerPort int
		monit                 JobSupervisor
		timeService           *fakeclock.FakeClock
		serviceManager        *servicemanagerfakes.FakeServiceManager
	)

	var jobFailureServerPort = 5000

	getJobFailureServerPort := func() int {
		jobFailureServerPort++
		return jobFailureServerPort
	}

	BeforeEach(func() {
		// go-smtp logs debug messages
		log.SetOutput(GinkgoWriter)

		fs = fakesys.NewFakeFileSystem()
		runner = fakesys.NewFakeCmdRunner()
		client = fakemonit.NewFakeMonitClient()
		logger = boshlog.NewLogger(boshlog.LevelNone)
		dirProvider = boshdir.NewProvider("/var/vcap")
		jobFailuresServerPort = getJobFailureServerPort()
		timeService = fakeclock.NewFakeClock(time.Now())
		serviceManager = &servicemanagerfakes.FakeServiceManager{}

		monit = NewMonitJobSupervisor(
			fs,
			runner,
			client,
			logger,
			dirProvider,
			jobFailuresServerPort,
			MonitReloadOptions{
				MaxTries:               3,
				MaxCheckTries:          10,
				DelayBetweenCheckTries: 0 * time.Millisecond,
			},
			timeService,
			serviceManager,
		)
	})

	doJobFailureEmail := func(email string, port int) error {
		conn, err := smtp.Dial(fmt.Sprintf("localhost:%d", port))
		for err != nil {
			conn, err = smtp.Dial(fmt.Sprintf("localhost:%d", port))
		}

		err = conn.Mail("sender@example.org")
		Expect(err).NotTo(HaveOccurred())
		err = conn.Rcpt("recipient@example.net")
		Expect(err).NotTo(HaveOccurred())
		writeCloser, err := conn.Data()
		if err != nil {
			return err
		}

		defer writeCloser.Close() //nolint:errcheck

		buf := bytes.NewBufferString(fmt.Sprintf("%s\r\n", email))
		_, err = buf.WriteTo(writeCloser)
		if err != nil {
			return err
		}

		return nil
	}

	Describe("Reload", func() {
		It("waits until the job is reloaded", func() {
			client.Incarnations = []int{1, 1, 1, 2, 3}
			client.StatusStatus = fakemonit.FakeMonitStatus{
				Services: []boshmonit.Service{
					boshmonit.Service{Monitored: true, Status: "failing"},
					boshmonit.Service{Monitored: true, Status: "running"},
				},
				Incarnation: 1,
			}

			err := monit.Reload()
			Expect(err).ToNot(HaveOccurred())

			Expect(serviceManager.KillCallCount()).To(Equal(1))
			Expect(serviceManager.StartCallCount()).To(Equal(1))
			Expect(client.StatusCalledTimes).To(Equal(4))
		})

		It("returns error after monit reloading X times, each time checking incarnation Y times", func() {
			for i := 0; i < 100; i++ {
				client.Incarnations = append(client.Incarnations, 1)
			}

			client.StatusStatus = fakemonit.FakeMonitStatus{
				Services: []boshmonit.Service{
					boshmonit.Service{Monitored: true, Status: "failing"},
					boshmonit.Service{Monitored: true, Status: "running"},
				},
				Incarnation: 1,
			}

			err := monit.Reload()
			Expect(err).To(HaveOccurred())

			Expect(serviceManager.KillCallCount()).To(Equal(3))
			Expect(serviceManager.StartCallCount()).To(Equal(3))

			Expect(client.StatusCalledTimes).To(Equal(1 + 30)) // old incarnation + new incarnation checks
		})

		It("is successful if the incarnation id is different (does not matter if < or >)", func() {
			client.Incarnations = []int{2, 2, 1} // different and less than old one
			client.StatusStatus = fakemonit.FakeMonitStatus{
				Services: []boshmonit.Service{
					boshmonit.Service{Monitored: true, Status: "failing"},
					boshmonit.Service{Monitored: true, Status: "running"},
				},
				Incarnation: 2,
			}

			err := monit.Reload()
			Expect(err).ToNot(HaveOccurred())

			Expect(serviceManager.KillCallCount()).To(Equal(1))
			Expect(serviceManager.StartCallCount()).To(Equal(1))
			Expect(client.StatusCalledTimes).To(Equal(3))
		})

		Context("when fetching the incarnation fails", func() {
			Context("before reloading monit", func() {
				BeforeEach(func() {
					client.StatusErr = errors.New("boom")
				})

				It("returns the error", func() {
					err := monit.Reload()
					Expect(err).To(HaveOccurred())
				})
			})

			Context("after reloading monit", func() {
				BeforeEach(func() {
					client.StatusStub = func() (boshmonit.Status, error) {
						if client.StatusCalledTimes == 1 {
							return fakemonit.FakeMonitStatus{Incarnation: 2}, nil
						}

						return nil, errors.New("boom")
					}
				})

				It("continues to retry fetching the incarnation", func() {
					err := monit.Reload()
					Expect(err).To(HaveOccurred())

					Expect(serviceManager.KillCallCount()).To(Equal(3))
					Expect(serviceManager.StartCallCount()).To(Equal(3))

					Expect(client.StatusCalledTimes).To(Equal(1 + 30)) // old incarnation + new incarnation checks
				})
			})
		})
	})

	Describe("Start", func() {
		It("start starts each monit service in group vcap", func() {
			client.ServicesInGroupServices = []string{"fake-service"}

			err := monit.Start()
			Expect(err).ToNot(HaveOccurred())

			Expect(client.ServicesInGroupName).To(Equal("vcap"))
			Expect(len(client.StartServiceNames)).To(Equal(1))
			Expect(client.StartServiceNames[0]).To(Equal("fake-service"))
		})

		It("deletes stopped file", func() {
			err := fs.MkdirAll("/var/vcap/monit/stopped", os.FileMode(0755))
			Expect(err).NotTo(HaveOccurred())
			err = fs.WriteFileString("/var/vcap/monit/stopped", "")
			Expect(err).NotTo(HaveOccurred())

			err = monit.Start()
			Expect(err).ToNot(HaveOccurred())
			Expect(fs.FileExists("/var/vcap/monit/stopped")).ToNot(BeTrue())
		})

		It("does not fail if stopped file is not present", func() {
			err := monit.Start()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Stop", func() {
		It("stop stops each monit service in group vcap", func() {
			client.ServicesInGroupServices = []string{"fake-service"}

			err := monit.Stop()
			Expect(err).ToNot(HaveOccurred())

			Expect(client.ServicesInGroupName).To(Equal("vcap"))
			Expect(len(client.StopServiceNames)).To(Equal(1))
			Expect(client.StopServiceNames[0]).To(Equal("fake-service"))
		})

		It("creates stopped file", func() {
			err := monit.Stop()
			Expect(err).ToNot(HaveOccurred())
			Expect(fs.FileExists("/var/vcap/monit/stopped")).To(BeTrue())
		})
	})

	Describe("StopAndWait", func() {
		It("stop stops each monit service in group vcap", func() {
			err := monit.StopAndWait()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(runner.RunCommands)).To(Equal(1))
			Expect(runner.RunCommands[0]).To(Equal([]string{"monit", "stop", "-g", "vcap"}))
		})

		It("stops", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestData := make(map[string]string)
				resBody := readFixture("monit/test_assets/monit_status_stopped.xml")

				if r.URL.Path == "/_status2" {
					_, err := w.Write(resBody)
					Expect(err).NotTo(HaveOccurred())
				} else {
					Expect(r.Method).To(Equal("POST"))
					requestData["action"] = r.PostFormValue("action")
				}
			})

			ts := httptest.NewServer(handler)
			defer ts.Close()

			url := ts.Listener.Addr().String()
			client := boshmonit.NewHTTPClient(
				url,
				"fake-user",
				"fake-pass",
				http.DefaultClient,
				http.DefaultClient,
				logger,
			)

			monit := NewMonitJobSupervisor(
				fs,
				runner,
				client,
				logger,
				dirProvider,
				jobFailuresServerPort,
				MonitReloadOptions{
					MaxTries:               3,
					MaxCheckTries:          10,
					DelayBetweenCheckTries: 0 * time.Millisecond,
				},
				timeService,
				serviceManager,
			)

			err := monit.StopAndWait()
			Expect(err).To(BeNil())
		})

		Describe("Waiting for pending services", func() {
			It("waits for services to not be pending before attempting to stop", func() {
				client.StatusStatus = fakemonit.FakeMonitStatus{
					Services: []boshmonit.Service{
						{Monitored: false, Name: "foo", Status: "unknown", Pending: true},
						{Monitored: true, Name: "bar", Status: "unknown", Pending: false},
					},
				}

				errchan := make(chan error)
				go func() {
					errchan <- monit.StopAndWait()
				}()

				Eventually(timeService.WatcherCount).Should(Equal(2)) // we hit the sleep

				// never called stop since 2 jobs pending
				Expect(len(runner.RunCommands)).To(Equal(0))

				client.StatusStatus = fakemonit.FakeMonitStatus{
					Services: []boshmonit.Service{
						{Monitored: false, Name: "foo", Status: "unknown", Pending: false},
						{Monitored: true, Name: "bar", Status: "unknown", Pending: true},
					},
				}
				timeService.Increment(2 * time.Minute)

				Eventually(timeService.WatcherCount).Should(Equal(2)) // we hit the sleep

				// never called stop since 1 job pending
				Expect(len(runner.RunCommands)).To(Equal(0))

				client.StatusStatus = fakemonit.FakeMonitStatus{
					Services: []boshmonit.Service{
						{Monitored: false, Name: "foo", Status: "unknown", Pending: false},
						{Monitored: false, Name: "bar", Status: "unknown", Pending: false},
					},
				}
				timeService.Increment(2 * time.Minute)

				Eventually(errchan).Should(Receive(BeNil()))
				Expect(len(runner.RunCommands)).To(Equal(1))
				Expect(runner.RunCommands[0]).To(Equal([]string{"monit", "stop", "-g", "vcap"}))
			})

			It("times out if services take too long to no longer be pending", func() {
				client.StatusStatus = fakemonit.FakeMonitStatus{
					Services: []boshmonit.Service{
						{Monitored: false, Name: "foo", Status: "unknown", Pending: true},
					},
				}

				errchan := make(chan error)
				go func() {
					errchan <- monit.StopAndWait()
				}()

				failureMessage := "Timed out waiting for services 'foo' to no longer be pending after 5 minutes"

				advanceTime(timeService, 10*time.Minute, 2)
				Eventually(timeService.WatcherCount).Should(Equal(0))
				Eventually(errchan).Should(Receive(Equal(errors.New(failureMessage))))
				Expect(len(runner.RunCommands)).To(Equal(0)) // never called 'monit stop'
			})

			It("uses the same timer for waiting for pending and waiting for services to stop", func() {
				client.StatusStatus = fakemonit.FakeMonitStatus{
					Services: []boshmonit.Service{
						{Monitored: false, Name: "foo", Status: "unknown", Pending: true},
					},
				}

				errchan := make(chan error)
				go func() {
					errchan <- monit.StopAndWait()
				}()

				Eventually(timeService.WatcherCount).Should(Equal(2)) // we hit the pending sleep

				client.StatusStatus = fakemonit.FakeMonitStatus{
					Services: []boshmonit.Service{
						{Monitored: true, Name: "foo", Status: "unknown", Pending: false},
					},
				}
				timeService.Increment(3 * time.Minute)

				Eventually(timeService.WatcherCount).Should(Equal(2)) // we hit the stop sleep

				timeService.Increment(3 * time.Minute)

				Eventually(errchan).Should(Receive(Equal(errors.New("Timed out waiting for services 'foo' to stop after 5 minutes"))))
			})
		})

		Context("when a status request errors", func() {
			It("exits with an error message if it's waiting for services to no longer be pending", func() {
				client.StatusStatus = fakemonit.FakeMonitStatus{
					Services: []boshmonit.Service{
						{Monitored: true, Name: "foo", Status: "unknown", Pending: true},
					},
				}

				errchan := make(chan error)
				go func() {
					errchan <- monit.StopAndWait()
				}()

				Eventually(timeService.WatcherCount).Should(Equal(2)) // we hit the sleep
				client.StatusErr = errors.New("Error message")

				timeService.Increment(5 * time.Minute)

				Eventually(func() string {
					err := <-errchan
					return err.Error()
				}).Should(Equal("Getting monit status: Error message"))
				Expect(len(runner.RunCommands)).To(Equal(0)) // never called 'monit stop', the right loop is failing
			})

			It("exits with an error message if it's waiting for services to stop", func() {
				client.StatusStatus = fakemonit.FakeMonitStatus{
					Services: []boshmonit.Service{
						{Monitored: true, Name: "foo", Status: "unknown", Pending: false},
					},
				}

				errchan := make(chan error)
				go func() {
					errchan <- monit.StopAndWait()
				}()

				Eventually(timeService.WatcherCount).Should(Equal(2)) // we hit the sleep
				client.StatusErr = errors.New("Error message")

				timeService.Increment(5 * time.Minute)

				Eventually(func() string {
					err := <-errchan
					return err.Error()
				}).Should(Equal("Getting monit status: Error message"))
				Expect(len(runner.RunCommands)).To(Equal(1)) // called 'monit stop', the right loop is failing
			})
		})

		Context("when a stop service errors", func() {
			It("exits with an error message", func() {
				fakeErrorResult := fakesys.FakeCmdResult{
					Error: errors.New("test error result"),
				}

				runner.AddCmdResult("monit stop -g vcap", fakeErrorResult)

				err := monit.StopAndWait()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(fmt.Sprintf("%s%s", "Stop all services: ", fakeErrorResult.Error)))
			})
		})

		Context("when a service is in error state", func() {
			It("exits with an error message", func() {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestData := make(map[string]string)
					resBody := readFixture("monit/test_assets/monit_status_errored.xml")

					if r.URL.Path == "/_status2" {
						_, err := w.Write(resBody)
						Expect(err).NotTo(HaveOccurred())
					} else {
						Expect(r.Method).To(Equal("POST"))
						requestData["action"] = r.PostFormValue("action")
					}
				})

				ts := httptest.NewServer(handler)
				defer ts.Close()

				url := ts.Listener.Addr().String()
				client := boshmonit.NewHTTPClient(
					url,
					"fake-user",
					"fake-pass",
					http.DefaultClient,
					http.DefaultClient,
					logger,
				)

				monit := NewMonitJobSupervisor(
					fs,
					runner,
					client,
					logger,
					dirProvider,
					jobFailuresServerPort,
					MonitReloadOptions{
						MaxTries:               3,
						MaxCheckTries:          10,
						DelayBetweenCheckTries: 0 * time.Millisecond,
					},
					timeService,
					serviceManager,
				)

				err := monit.StopAndWait()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Stopping services '[test-service]' errored"))
			})
		})

		Context("when a service takes too long to stop", func() {
			It("exits with an error after a timeout", func() {
				statusRequests := 0
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestData := make(map[string]string)

					if r.URL.Path == "/_status2" {
						statusRequests++
						if statusRequests == 1 {
							_, err := w.Write(readFixture("monit/test_assets/monit_status_running.xml"))
							Expect(err).NotTo(HaveOccurred())
						} else {
							_, err := w.Write(readFixture("monit/test_assets/monit_status_multiple.xml"))
							Expect(err).NotTo(HaveOccurred())
						}
					} else {
						Expect(r.Method).To(Equal("POST"))
						requestData["action"] = r.PostFormValue("action")
					}
				})

				ts := httptest.NewServer(handler)
				defer ts.Close()

				url := ts.Listener.Addr().String()
				client := boshmonit.NewHTTPClient(
					url,
					"fake-user",
					"fake-pass",
					http.DefaultClient,
					http.DefaultClient,
					logger,
				)

				monit := NewMonitJobSupervisor(
					fs,
					runner,
					client,
					logger,
					dirProvider,
					jobFailuresServerPort,
					MonitReloadOptions{},
					timeService,
					serviceManager,
				)

				errchan := make(chan error)
				go func() {
					errchan <- monit.StopAndWait()
				}()

				failureMessage := "Timed out waiting for services 'unmonitored-start-pending, initializing, running, running-stop-pending, unmonitored-stop-pending, failing' to stop after 5 minutes"

				advanceTime(timeService, 5*time.Minute, 2)
				Eventually(timeService.WatcherCount).Should(Equal(0))
				Eventually(errchan).Should(Receive(Equal(errors.New(failureMessage))))
				Expect(statusRequests).To(Equal(3))
			})
		})

		It("creates stopped file", func() {
			err := monit.StopAndWait()
			Expect(err).ToNot(HaveOccurred())
			Expect(fs.FileExists("/var/vcap/monit/stopped")).To(BeTrue())
		})
	})

	Describe("Status", func() {
		It("status returns running when all services are monitored and running", func() {
			client.StatusStatus = fakemonit.FakeMonitStatus{
				Services: []boshmonit.Service{
					boshmonit.Service{Monitored: true, Status: "running"},
					boshmonit.Service{Monitored: true, Status: "running"},
				},
			}

			status := monit.Status()
			Expect("running").To(Equal(status))
		})

		It("status returns failing when all services are monitored and at least one service is failing", func() {
			client.StatusStatus = fakemonit.FakeMonitStatus{
				Services: []boshmonit.Service{
					boshmonit.Service{Monitored: true, Status: "failing"},
					boshmonit.Service{Monitored: true, Status: "running"},
				},
			}

			status := monit.Status()
			Expect("failing").To(Equal(status))
		})

		It("status returns failing when at least one service is not monitored", func() {
			client.StatusStatus = fakemonit.FakeMonitStatus{
				Services: []boshmonit.Service{
					boshmonit.Service{Monitored: false, Status: "running"},
					boshmonit.Service{Monitored: true, Status: "running"},
				},
			}

			status := monit.Status()
			Expect("failing").To(Equal(status))
		})

		It("status returns start when at least one service is starting", func() {
			client.StatusStatus = fakemonit.FakeMonitStatus{
				Services: []boshmonit.Service{
					boshmonit.Service{Monitored: true, Status: "failing"},
					boshmonit.Service{Monitored: true, Status: "starting"},
					boshmonit.Service{Monitored: true, Status: "running"},
				},
			}

			status := monit.Status()
			Expect("starting").To(Equal(status))
		})

		It("status returns unknown when error", func() {
			client.StatusErr = errors.New("fake-monit-client-error")

			status := monit.Status()
			Expect("unknown").To(Equal(status))
		})

		It("returns running if there are no vcap service", func() {
			client.StatusStatus = fakemonit.FakeMonitStatus{
				Services: []boshmonit.Service{},
			}

			status := monit.Status()
			Expect(status).To(Equal("running"))
		})

		It("returns stopped if there are stop was called before", func() {
			client.StatusStatus = fakemonit.FakeMonitStatus{
				Services: []boshmonit.Service{},
			}

			err := fs.MkdirAll("/var/vcap/monit/stopped", os.FileMode(0755))
			Expect(err).NotTo(HaveOccurred())
			err = fs.WriteFileString("/var/vcap/monit/stopped", "")
			Expect(err).NotTo(HaveOccurred())

			status := monit.Status()
			Expect(status).To(Equal("stopped"))
		})
	})

	Describe("Processes", func() {
		It("returns all processes", func() {
			client.StatusStatus = fakemonit.FakeMonitStatus{
				Services: []boshmonit.Service{
					boshmonit.Service{
						Name:                 "fake-service-1",
						Monitored:            true,
						Status:               "running",
						Uptime:               1234,
						MemoryPercentTotal:   0.4,
						MemoryKilobytesTotal: 100,
						CPUPercentTotal:      0.5,
					},
					boshmonit.Service{
						Name:                 "fake-service-2",
						Monitored:            true,
						Status:               "failing",
						Uptime:               1235,
						MemoryPercentTotal:   0.6,
						MemoryKilobytesTotal: 200,
						CPUPercentTotal:      0.7,
					},
				},
			}

			processes, err := monit.Processes()
			Expect(err).ToNot(HaveOccurred())
			Expect(processes).To(Equal([]Process{
				Process{
					Name:  "fake-service-1",
					State: "running",
					Uptime: UptimeVitals{
						Secs: 1234,
					},
					Memory: MemoryVitals{
						Kb:      100,
						Percent: 0.4,
					},
					CPU: CPUVitals{
						Total: 0.5,
					},
				},
				Process{
					Name:  "fake-service-2",
					State: "failing",
					Uptime: UptimeVitals{
						Secs: 1235,
					},
					Memory: MemoryVitals{
						Kb:      200,
						Percent: 0.6,
					},
					CPU: CPUVitals{
						Total: 0.7,
					},
				},
			}))
		})

		It("returns error when failing to get service status", func() {
			client.StatusErr = errors.New("fake-monit-client-error")

			processes, err := monit.Processes()
			Expect(err).To(HaveOccurred())
			Expect(processes).To(BeEmpty())
		})
	})

	Describe("MonitorJobFailures", func() {
		It("monitor job failures", func() {
			var handledAlert boshalert.MonitAlert

			failureHandler := func(alert boshalert.MonitAlert) (err error) {
				handledAlert = alert
				return
			}

			go func() {
				defer GinkgoRecover()

				err := monit.MonitorJobFailures(failureHandler)
				Expect(err).NotTo(HaveOccurred())
			}()

			msg := `Message-id: <1304319946.0@localhost>
 Service: nats
 Event: does not exist
 Action: restart
 Date: Sun, 22 May 2011 20:07:41 +0500
 Description: process is not running`

			err := doJobFailureEmail(msg, jobFailuresServerPort)
			Expect(err).ToNot(HaveOccurred())

			Expect(handledAlert).To(Equal(boshalert.MonitAlert{
				ID:          "1304319946.0@localhost",
				Service:     "nats",
				Event:       "does not exist",
				Action:      "restart",
				Date:        "Sun, 22 May 2011 20:07:41 +0500",
				Description: "process is not running",
			}))
		})

		It("ignores other emails", func() {
			var didHandleAlert bool

			failureHandler := func(alert boshalert.MonitAlert) (err error) {
				didHandleAlert = true
				return
			}

			go func() {
				defer GinkgoRecover()

				err := monit.MonitorJobFailures(failureHandler)
				Expect(err).NotTo(HaveOccurred())
			}()

			err := doJobFailureEmail(`fake-other-email`, jobFailuresServerPort)
			Expect(err).ToNot(HaveOccurred())
			Expect(didHandleAlert).To(BeFalse())
		})
	})

	Describe("AddJob", func() {
		BeforeEach(func() {
			err := fs.WriteFileString("/some/config/path", "fake-config")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when reading configuration from config path succeeds", func() {
			Context("when writing job configuration succeeds", func() {
				It("returns no error because monit can track added job in jobs directory", func() {
					err := monit.AddJob("router", 0, "/some/config/path")
					Expect(err).ToNot(HaveOccurred())

					writtenConfig, err := fs.ReadFileString(
						dirProvider.MonitJobsDir() + "/0000_router.monitrc")
					Expect(err).ToNot(HaveOccurred())
					Expect(writtenConfig).To(Equal("fake-config"))
				})
			})

			Context("when writing job configuration fails", func() {
				It("returns error", func() {
					fs.WriteFileError = errors.New("fake-write-error")

					err := monit.AddJob("router", 0, "/some/config/path")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-write-error"))
				})
			})
		})

		Context("when reading configuration from config path fails", func() {
			It("returns error", func() {
				fs.ReadFileError = errors.New("fake-read-error")

				err := monit.AddJob("router", 0, "/some/config/path")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-read-error"))
			})
		})
	})

	Describe("RemoveAllJobs", func() {
		Context("when jobs directory removal succeeds", func() {
			It("does not return error because all jobs are removed from monit", func() {
				jobsDir := dirProvider.MonitJobsDir()
				jobBasename := "/0000_router.monitrc"
				err := fs.WriteFileString(jobsDir+jobBasename, "fake-added-job")
				Expect(err).NotTo(HaveOccurred())

				err = monit.RemoveAllJobs()
				Expect(err).ToNot(HaveOccurred())

				Expect(fs.FileExists(jobsDir)).To(BeFalse())
				Expect(fs.FileExists(jobsDir + jobBasename)).To(BeFalse())
			})
		})

		Context("when jobs directory removal fails", func() {
			It("returns error if removing jobs directory fails", func() {
				fs.RemoveAllStub = func(_ string) error {
					return errors.New("fake-remove-all-error")
				}

				err := monit.RemoveAllJobs()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-remove-all-error"))
			})
		})
	})

	Describe("Unmonitor", func() {
		BeforeEach(func() {
			client.ServicesInGroupServices = []string{"fake-srv-1", "fake-srv-2", "fake-srv-3"}
			client.UnmonitorServiceErrs = []error{nil, nil, nil}
		})

		Context("when all services succeed to be unmonitored", func() {
			It("returns no error because all services got unmonitored", func() {
				err := monit.Unmonitor()
				Expect(err).ToNot(HaveOccurred())

				Expect(client.ServicesInGroupName).To(Equal("vcap"))
				Expect(client.UnmonitorServiceNames).To(Equal(
					[]string{"fake-srv-1", "fake-srv-2", "fake-srv-3"}))
			})
		})

		Context("when at least one service fails to be unmonitored", func() {
			BeforeEach(func() {
				client.UnmonitorServiceErrs = []error{
					nil, errors.New("fake-unmonitor-error"), nil,
				}
			})

			It("returns first unmonitor error", func() {
				err := monit.Unmonitor()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-unmonitor-error"))
			})

			It("only tries to unmonitor services before the first unmonitor error", func() {
				err := monit.Unmonitor()
				Expect(err).To(HaveOccurred())
				Expect(client.ServicesInGroupName).To(Equal("vcap"))
				Expect(client.UnmonitorServiceNames).To(Equal([]string{"fake-srv-1", "fake-srv-2"}))
			})
		})

		Context("when failed retrieving list of services", func() {
			It("returns error", func() {
				client.ServicesInGroupErr = errors.New("fake-services-error")

				err := monit.Unmonitor()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-services-error"))
			})
		})
	})
})

func advanceTime(timeService *fakeclock.FakeClock, duration time.Duration, watcherCount int) {
	Eventually(timeService.WatcherCount).Should(Equal(watcherCount))
	timeService.Increment(duration)
}
