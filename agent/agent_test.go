package agent_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakeuuid "github.com/cloudfoundry/bosh-utils/uuid/fakes"

	"github.com/cloudfoundry/bosh-agent/v2/agent"
	"github.com/cloudfoundry/bosh-agent/v2/agent/agentfakes"
	boshalert "github.com/cloudfoundry/bosh-agent/v2/agent/alert"
	boshas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	fakeas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec/fakes"
	fakeagent "github.com/cloudfoundry/bosh-agent/v2/agent/fakes"
	boshhandler "github.com/cloudfoundry/bosh-agent/v2/handler"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/fakes"
	fakembus "github.com/cloudfoundry/bosh-agent/v2/mbus/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
	boshvitals "github.com/cloudfoundry/bosh-agent/v2/platform/vitals"
	"github.com/cloudfoundry/bosh-agent/v2/platform/vitals/vitalsfakes"
	fakesettings "github.com/cloudfoundry/bosh-agent/v2/settings/fakes"
)

func init() { //nolint:funlen,gochecknoinits
	Describe("Agent", func() {
		var (
			logger           boshlog.Logger
			handler          *fakembus.FakeHandler
			platform         *platformfakes.FakePlatform
			actionDispatcher *fakeagent.FakeActionDispatcher
			jobSupervisor    *fakejobsuper.FakeJobSupervisor
			specService      *fakeas.FakeV1Service
			settingsService  *fakesettings.FakeSettingsService
			uuidGenerator    *fakeuuid.FakeGenerator
			timeService      *fakeclock.FakeClock
			vitalService     *vitalsfakes.FakeService
			startManager     *agentfakes.FakeStartManager

			boshAgent agent.Agent
		)

		BeforeEach(func() {
			logger = boshlog.NewLogger(boshlog.LevelNone)
			handler = &fakembus.FakeHandler{}
			platform = &platformfakes.FakePlatform{}
			actionDispatcher = &fakeagent.FakeActionDispatcher{}
			jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
			specService = fakeas.NewFakeV1Service()
			settingsService = &fakesettings.FakeSettingsService{}
			uuidGenerator = &fakeuuid.FakeGenerator{}
			timeService = fakeclock.NewFakeClock(time.Now())
			vitalService = &vitalsfakes.FakeService{}
			startManager = &agentfakes.FakeStartManager{}
			startManager.CanStartReturns(true)

			platform.GetVitalsServiceReturns(vitalService)

			boshAgent = agent.New(
				logger,
				handler,
				platform,
				actionDispatcher,
				jobSupervisor,
				specService,
				5*time.Millisecond,
				settingsService,
				uuidGenerator,
				timeService,
				startManager,
			)
		})

		Describe("Run", func() {
			It("Registers a start with the startManager", func() {
				err := boshAgent.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(startManager.RegisterStartCallCount()).To(Equal(1))
			})

			It("lets dispatcher handle requests arriving via handler", func() {
				err := boshAgent.Run()
				Expect(err).ToNot(HaveOccurred())

				expectedResp := boshhandler.NewValueResponse("pong")
				actionDispatcher.DispatchResp = expectedResp

				req := boshhandler.NewRequest("fake-reply", "fake-action", []byte("fake-payload"), 0)
				resp := handler.RunFunc(req)

				Expect(actionDispatcher.DispatchReq).To(Equal(req))
				Expect(resp).To(Equal(expectedResp))
			})

			It("resumes persistent actions *before* dispatching new requests", func() {
				resumedBeforeStartingToDispatch := false
				handler.RunCallBack = func() {
					resumedBeforeStartingToDispatch = actionDispatcher.ResumedPreviouslyDispatchedTasks
				}

				err := boshAgent.Run()
				Expect(err).ToNot(HaveOccurred())
				Expect(resumedBeforeStartingToDispatch).To(BeTrue())
			})

			Context("when heartbeats can be sent", func() {
				BeforeEach(func() {
					handler.KeepOnRunning()
				})

				BeforeEach(func() {
					jobName := "fake-job"
					nodeID := "node-id"
					jobIndex := 1
					specService.Spec = boshas.V1ApplySpec{
						Deployment: "FakeDeployment",
						JobSpec:    boshas.JobSpec{Name: &jobName},
						Index:      &jobIndex,
						NodeID:     nodeID,
					}

					jobSupervisor.StatusStatus = "fake-state"

					vitalService.GetReturns(boshvitals.Vitals{
						Load: []string{"a", "b", "c"},
					}, nil)
				})

				expectedJobName := "fake-job"
				expectedJobIndex := 1
				expectedNodeID := "node-id"
				expectedHb := agent.Heartbeat{
					Deployment: "FakeDeployment",
					Job:        &expectedJobName,
					Index:      &expectedJobIndex,
					JobState:   "fake-state",
					NodeID:     expectedNodeID,
					Vitals:     boshvitals.Vitals{Load: []string{"a", "b", "c"}},
				}

				It("sends initial heartbeat", func() {
					// Configure periodic heartbeat every 5 hours
					// so that we are sure that we will not receive it
					boshAgent = agent.New(
						logger,
						handler,
						platform,
						actionDispatcher,
						jobSupervisor,
						specService,
						5*time.Hour,
						settingsService,
						uuidGenerator,
						timeService,
						startManager,
					)

					// Immediately exit after sending initial heartbeat
					handler.SendErr = errors.New("stop")

					err := boshAgent.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("stop"))

					Expect(handler.SendInputs()).To(Equal([]fakembus.SendInput{
						{
							Target:  boshhandler.HealthMonitor,
							Topic:   boshhandler.Heartbeat,
							Message: expectedHb,
						},
					}))

					Expect(jobSupervisor.GetHealthRecorded()).To(Equal(1))
				})

				It("sends periodic heartbeats, with retry", func() {
					sentRequests := 0
					handler.SendCallback = func(_ fakembus.SendInput) {
						sentRequests++
						if sentRequests == 3 {
							handler.SendErr = errors.New("disconnect")
						}
						if sentRequests == 4 {
							handler.SendErr = nil
						}
						if sentRequests == 5 {
							handler.SendErr = errors.New("stop")
						}
					}

					err := boshAgent.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("stop"))

					inputs := handler.SendInputs()
					Expect(len(inputs)).To(BeNumerically(">=", 15))
					for _, input := range inputs {
						Expect(input).To(Equal(fakembus.SendInput{
							Target:  boshhandler.HealthMonitor,
							Topic:   boshhandler.Heartbeat,
							Message: expectedHb,
						}))
					}
					Expect(jobSupervisor.GetHealthRecorded()).To(BeNumerically(">=", 3))
				})

				Context("when the boshAgent may not be rebooted", func() {
					BeforeEach(func() {
						startManager.CanStartReturns(false)
					})

					It("stops the boot process and returns an error", func() {
						err := boshAgent.Run()
						Expect(err).To(HaveOccurred())
					})
				})
			})

			Context("when the boshAgent fails to get job spec for a heartbeat", func() {
				BeforeEach(func() {
					specService.GetErr = errors.New("fake-spec-service-error")
					handler.KeepOnRunning()
				})

				It("returns the error", func() {
					err := boshAgent.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-spec-service-error"))
				})
			})

			Context("when the boshAgent fails to get vitals for a heartbeat", func() {
				BeforeEach(func() {
					vitalService.GetReturns(boshvitals.Vitals{}, errors.New("fake-vitals-service-error"))
					handler.KeepOnRunning()
				})

				It("returns the error", func() {
					err := boshAgent.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-vitals-service-error"))
				})
			})

			It("sends job monitoring alerts to health manager", func() {
				handler.KeepOnRunning()

				monitAlert := boshalert.MonitAlert{
					ID:          "fake-monit-alert",
					Service:     "fake-service",
					Event:       "fake-event",
					Action:      "fake-action",
					Date:        "Sun, 22 May 2011 20:07:41 +0500",
					Description: "fake-description",
				}
				jobSupervisor.JobFailureAlert = &monitAlert

				// Fail the first time handler.Send is called for an alert (ignore heartbeats)
				handler.SendCallback = func(input fakembus.SendInput) {
					if input.Topic == boshhandler.Alert {
						handler.SendErr = errors.New("stop")
					}
				}

				err := boshAgent.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("stop"))

				expectedAlert := boshalert.Alert{
					ID:        "fake-monit-alert",
					Severity:  boshalert.SeverityDefault,
					Title:     "fake-service - fake-event - fake-action",
					Summary:   "fake-description",
					CreatedAt: int64(1306076861),
				}

				Expect(handler.SendInputs()).To(ContainElement(fakembus.SendInput{
					Target:  boshhandler.HealthMonitor,
					Topic:   boshhandler.Alert,
					Message: expectedAlert,
				}))
			})
		})
	})
}
