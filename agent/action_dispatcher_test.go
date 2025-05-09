package agent_test

import (
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshassert "github.com/cloudfoundry/bosh-utils/assert"
	fakes "github.com/cloudfoundry/bosh-utils/logger/loggerfakes"

	"github.com/cloudfoundry/bosh-agent/v2/agent"
	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	fakeaction "github.com/cloudfoundry/bosh-agent/v2/agent/action/fakes"
	boshtask "github.com/cloudfoundry/bosh-agent/v2/agent/task"
	faketask "github.com/cloudfoundry/bosh-agent/v2/agent/task/fakes"
	boshhandler "github.com/cloudfoundry/bosh-agent/v2/handler"
)

func init() { //nolint:funlen,gochecknoinits
	Describe("actionDispatcher", func() {
		var (
			logger        *fakes.FakeLogger
			taskService   *faketask.FakeService
			taskManager   *faketask.FakeManager
			actionFactory *fakeaction.FakeFactory
			actionRunner  *fakeaction.FakeRunner
			dispatcher    agent.ActionDispatcher
		)

		BeforeEach(func() {
			logger = &fakes.FakeLogger{}
			taskService = faketask.NewFakeService()
			taskManager = faketask.NewFakeManager()
			actionFactory = fakeaction.NewFakeFactory()
			actionRunner = &fakeaction.FakeRunner{}
			dispatcher = agent.NewActionDispatcher(logger, taskService, taskManager, actionFactory, actionRunner)
		})

		It("responds with exception when the method is unknown", func() {
			actionFactory.RegisterActionErr("fake-action", errors.New("fake-create-error"))

			req := boshhandler.NewRequest("fake-reply", "fake-action", []byte{}, 0)
			resp := dispatcher.Dispatch(req)
			boshassert.MatchesJSONString(GinkgoT(), resp, `{"exception":{"message":"unknown message fake-action"}}`)
		})

		Context("Action Payload Logging", func() {
			var (
				action *fakeaction.TestAction
				req    boshhandler.Request
			)

			Context("action is loggable", func() {
				BeforeEach(func() {
					req = boshhandler.NewRequest("fake-reply", "fake-action", []byte("fake-payload"), 0)
					action = &fakeaction.TestAction{Loggable: true}
					actionFactory.RegisterAction("fake-action", action)
					dispatcher.Dispatch(req)
				})

				It("logs the payload", func() {
					Expect(logger.DebugWithDetailsCallCount()).To(Equal(1))
					_, message, args := logger.DebugWithDetailsArgsForCall(0)
					Expect(message).To(Equal("Payload"))
					Expect(args[0]).To(Equal(req.Payload))
				})
			})

			Context("action is not loggable", func() {
				BeforeEach(func() {
					req = boshhandler.NewRequest("fake-reply", "fake-action", []byte("fake-payload"), 0)
					action = &fakeaction.TestAction{Loggable: false}
					actionFactory.RegisterAction("fake-action", action)
					dispatcher.Dispatch(req)
				})

				It("does not log the payload", func() {
					Expect(logger.DebugWithDetailsCallCount()).To(Equal(0))
				})
			})
		})

		Context("when request contains protocol version and action is Asynchronous", func() {
			var (
				req       boshhandler.Request
				runAction *fakeaction.TestAction
			)

			BeforeEach(func() {
				runAction = &fakeaction.TestAction{Asynchronous: true}
				actionFactory.RegisterAction("fake-action", runAction)
			})

			It("passes protocol version zero to IsSynchronous", func() {
				req = boshhandler.NewRequest("fake-reply", "fake-action", []byte("fake-payload"), boshhandler.ProtocolVersion(0))

				dispatcher.Dispatch(req)

				_, err := taskService.StartedTasks["fake-generated-task-id"].Func()
				Expect(err).ToNot(HaveOccurred())

				Expect(actionRunner.RunAction).To(Equal(runAction))
				Expect(string(actionRunner.RunPayload)).To(Equal("fake-payload"))

				Expect(runAction.ProtocolVersion).To(Equal(action.ProtocolVersion(0)))
				Expect(actionRunner.RunProtocolVersion).To(Equal(action.ProtocolVersion(0)))
			})

			It("passes protocol version to IsSynchronous", func() {
				req = boshhandler.NewRequest("fake-reply", "fake-action", []byte("fake-payload"), boshhandler.ProtocolVersion(99))
				dispatcher.Dispatch(req)

				_, err := taskService.StartedTasks["fake-generated-task-id"].Func()
				Expect(err).ToNot(HaveOccurred())

				Expect(actionRunner.RunAction).To(Equal(runAction))
				Expect(string(actionRunner.RunPayload)).To(Equal("fake-payload"))

				Expect(runAction.ProtocolVersion).To(Equal(action.ProtocolVersion(99)))
				Expect(actionRunner.RunProtocolVersion).To(Equal(action.ProtocolVersion(99)))
			})
		})

		Context("when request contains protocol version and action is Synchronous", func() {
			var (
				req       boshhandler.Request
				runAction *fakeaction.TestAction
			)

			BeforeEach(func() {
				runAction = &fakeaction.TestAction{Asynchronous: false}
				actionFactory.RegisterAction("fake-action", runAction)
			})

			It("passes protocol version zero to IsSynchronous", func() {
				req = boshhandler.NewRequest("fake-reply", "fake-action", []byte("fake-payload"), boshhandler.ProtocolVersion(0))
				dispatcher.Dispatch(req)

				Expect(runAction.ProtocolVersion).To(Equal(action.ProtocolVersion(0)))
				Expect(actionRunner.RunProtocolVersion).To(Equal(action.ProtocolVersion(0)))
			})

			It("passes protocol version to IsSynchronous", func() {
				req = boshhandler.NewRequest("fake-reply", "fake-action", []byte("fake-payload"), boshhandler.ProtocolVersion(99))
				dispatcher.Dispatch(req)

				Expect(runAction.ProtocolVersion).To(Equal(action.ProtocolVersion(99)))
				Expect(actionRunner.RunProtocolVersion).To(Equal(action.ProtocolVersion(99)))
			})
		})

		Context("when action is synchronous", func() {
			var (
				req boshhandler.Request
			)

			BeforeEach(func() {
				req = boshhandler.NewRequest("fake-reply", "fake-action", []byte("fake-payload"), 0)
				actionFactory.RegisterAction("fake-action", &fakeaction.TestAction{Asynchronous: false})
			})

			It("handles synchronous action", func() {
				actionRunner.RunValue = "fake-value"

				resp := dispatcher.Dispatch(req)
				Expect(req.GetPayload()).To(Equal(actionRunner.RunPayload))
				Expect(boshhandler.NewValueResponse("fake-value")).To(Equal(resp))
			})

			It("handles synchronous action when err", func() {
				actionRunner.RunErr = errors.New("fake-run-error")

				resp := dispatcher.Dispatch(req)
				expectedJSON := fmt.Sprintf("{\"exception\":{\"message\":\"Action Failed %s: fake-run-error\"}}", req.Method)
				boshassert.MatchesJSONString(GinkgoT(), resp, expectedJSON)
			})
		})

		Context("when action is asynchronous", func() {
			var (
				req    boshhandler.Request
				action *fakeaction.TestAction
			)

			BeforeEach(func() {
				req = boshhandler.NewRequest("fake-reply", "fake-action", []byte("fake-payload"), 0)
				action = &fakeaction.TestAction{Asynchronous: true}
				actionFactory.RegisterAction("fake-action", action)
			})

			ItAllowsToCancelTask := func() {
				It("allows task to be cancelled", func() {
					dispatcher.Dispatch(req)

					err := taskService.StartedTasks["fake-generated-task-id"].Cancel()
					Expect(err).ToNot(HaveOccurred())

					Expect(action.Canceled).To(BeTrue())
				})

				It("returns error from cancelling task if canceling task fails", func() {
					action.CancelErr = errors.New("fake-cancel-err")
					dispatcher.Dispatch(req)

					err := taskService.StartedTasks["fake-generated-task-id"].Cancel()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-cancel-err"))
				})
			}

			Context("when action is not persistent", func() {
				BeforeEach(func() {
					action.Persistent = false
				})

				It("responds with task id and state", func() {
					resp := dispatcher.Dispatch(req)
					boshassert.MatchesJSONString(GinkgoT(), resp,
						`{"value":{"agent_task_id":"fake-generated-task-id","state":"running"}}`)
				})

				It("starts running created task", func() {
					dispatcher.Dispatch(req)
					Expect(len(taskService.StartedTasks)).To(Equal(1))
					Expect(taskService.StartedTasks["fake-generated-task-id"]).ToNot(BeNil())
				})

				It("returns create task error", func() {
					taskService.CreateTaskErr = errors.New("fake-create-task-error")
					resp := dispatcher.Dispatch(req)
					respJSON, err := json.Marshal(resp)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(respJSON)).To(ContainSubstring("fake-create-task-error"))
				})

				It("return run value to the task", func() {
					actionRunner.RunValue = "fake-value"
					dispatcher.Dispatch(req)

					value, err := taskService.StartedTasks["fake-generated-task-id"].Func()
					Expect(value).To(Equal("fake-value"))
					Expect(err).ToNot(HaveOccurred())

					Expect(actionRunner.RunAction).To(Equal(action))
					Expect(string(actionRunner.RunPayload)).To(Equal("fake-payload"))
				})

				It("returns run error to the task", func() {
					actionRunner.RunErr = errors.New("fake-run-error")
					dispatcher.Dispatch(req)

					value, err := taskService.StartedTasks["fake-generated-task-id"].Func()
					Expect(value).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-run-error"))

					Expect(actionRunner.RunAction).To(Equal(action))
					Expect(string(actionRunner.RunPayload)).To(Equal("fake-payload"))
				})

				ItAllowsToCancelTask()

				It("does not add task to task manager since it should not be resumed if agent is restarted", func() {
					dispatcher.Dispatch(req)
					taskInfos, _ := taskManager.GetInfos() //nolint:errcheck
					Expect(taskInfos).To(BeEmpty())
				})

				It("does not do anything after task finishes", func() {
					dispatcher.Dispatch(req)
					Expect(taskService.StartedTasks["fake-generated-task-id"].EndFunc).To(BeNil())
				})
			})

			Context("when action is persistent", func() {
				BeforeEach(func() {
					action.Persistent = true
				})

				It("responds with task id and state", func() {
					resp := dispatcher.Dispatch(req)
					boshassert.MatchesJSONString(GinkgoT(), resp,
						`{"value":{"agent_task_id":"fake-generated-task-id","state":"running"}}`)
				})

				It("starts running created task", func() {
					dispatcher.Dispatch(req)
					Expect(len(taskService.StartedTasks)).To(Equal(1))
					Expect(taskService.StartedTasks["fake-generated-task-id"]).ToNot(BeNil())
				})

				It("returns create task error", func() {
					taskService.CreateTaskErr = errors.New("fake-create-task-error")
					resp := dispatcher.Dispatch(req)
					respJSON, err := json.Marshal(resp)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(respJSON)).To(ContainSubstring("fake-create-task-error"))
				})

				It("return run value to the task", func() {
					actionRunner.RunValue = "fake-value"
					dispatcher.Dispatch(req)

					value, err := taskService.StartedTasks["fake-generated-task-id"].Func()
					Expect(value).To(Equal("fake-value"))
					Expect(err).ToNot(HaveOccurred())

					Expect(actionRunner.RunAction).To(Equal(action))
					Expect(string(actionRunner.RunPayload)).To(Equal("fake-payload"))
				})

				It("returns run error to the task", func() {
					actionRunner.RunErr = errors.New("fake-run-error")
					dispatcher.Dispatch(req)

					value, err := taskService.StartedTasks["fake-generated-task-id"].Func()
					Expect(value).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-run-error"))

					Expect(actionRunner.RunAction).To(Equal(action))
					Expect(string(actionRunner.RunPayload)).To(Equal("fake-payload"))
				})

				ItAllowsToCancelTask()

				It("adds task to task manager before task starts so that it could be resumed if agent is restarted", func() {
					dispatcher.Dispatch(req)               //nolint:errcheck
					taskInfos, _ := taskManager.GetInfos() //nolint:errcheck
					Expect(taskInfos).To(Equal([]boshtask.Info{
						boshtask.Info{
							TaskID:  "fake-generated-task-id",
							Method:  "fake-action",
							Payload: []byte("fake-payload"),
						},
					}))
				})

				It("removes task from task manager after task finishes", func() {
					dispatcher.Dispatch(req)
					taskService.StartedTasks["fake-generated-task-id"].EndFunc(boshtask.Task{ID: "fake-generated-task-id"})

					taskInfos, _ := taskManager.GetInfos() //nolint:errcheck
					Expect(taskInfos).To(BeEmpty())
				})

				It("does not start running created task if task manager cannot add task", func() {
					taskManager.AddInfoErr = errors.New("fake-add-task-info-error")

					resp := dispatcher.Dispatch(req)
					boshassert.MatchesJSONString(GinkgoT(), resp,
						`{"exception":{"message":"Action Failed fake-action: fake-add-task-info-error"}}`)

					Expect(len(taskService.StartedTasks)).To(Equal(0))
				})
			})
		})

		Describe("ResumePreviouslyDispatchedTasks", func() {
			var firstAction, secondAction *fakeaction.TestAction

			BeforeEach(func() {
				err := taskManager.AddInfo(boshtask.Info{
					TaskID:  "fake-task-id-1",
					Method:  "fake-action-1",
					Payload: []byte("fake-task-payload-1"),
				})
				Expect(err).ToNot(HaveOccurred())

				err = taskManager.AddInfo(boshtask.Info{
					TaskID:  "fake-task-id-2",
					Method:  "fake-action-2",
					Payload: []byte("fake-task-payload-2"),
				})
				Expect(err).ToNot(HaveOccurred())

				firstAction = &fakeaction.TestAction{}
				secondAction = &fakeaction.TestAction{}
			})

			It("calls resume on each task that was saved in a task manager", func() {
				actionFactory.RegisterAction("fake-action-1", firstAction)
				actionFactory.RegisterAction("fake-action-2", secondAction)

				dispatcher.ResumePreviouslyDispatchedTasks()
				Expect(len(taskService.StartedTasks)).To(Equal(2))

				{ // Check that first task executes first action
					actionRunner.ResumeValue = "fake-resume-value-1"
					value, err := taskService.StartedTasks["fake-task-id-1"].Func()
					Expect(err).ToNot(HaveOccurred())
					Expect(value).To(Equal("fake-resume-value-1"))
					Expect(actionRunner.ResumeAction).To(Equal(firstAction))
					Expect(string(actionRunner.ResumePayload)).To(Equal("fake-task-payload-1"))
				}

				{ // Check that second task executes second action
					actionRunner.ResumeValue = "fake-resume-value-2"
					value, err := taskService.StartedTasks["fake-task-id-2"].Func()
					Expect(err).ToNot(HaveOccurred())
					Expect(value).To(Equal("fake-resume-value-2"))
					Expect(actionRunner.ResumeAction).To(Equal(secondAction))
					Expect(string(actionRunner.ResumePayload)).To(Equal("fake-task-payload-2"))
				}
			})

			It("removes tasks from task manager after each task finishes", func() {
				actionFactory.RegisterAction("fake-action-1", firstAction)
				actionFactory.RegisterAction("fake-action-2", secondAction)

				dispatcher.ResumePreviouslyDispatchedTasks()
				Expect(len(taskService.StartedTasks)).To(Equal(2))

				// Simulate all tasks ending
				taskService.StartedTasks["fake-task-id-1"].EndFunc(boshtask.Task{ID: "fake-task-id-1"})
				taskService.StartedTasks["fake-task-id-2"].EndFunc(boshtask.Task{ID: "fake-task-id-2"})

				taskInfos, err := taskManager.GetInfos()
				Expect(err).ToNot(HaveOccurred())
				Expect(taskInfos).To(BeEmpty())
			})

			It("return resume error to each task", func() {
				actionFactory.RegisterAction("fake-action-1", firstAction)
				actionFactory.RegisterAction("fake-action-2", secondAction)

				dispatcher.ResumePreviouslyDispatchedTasks()
				Expect(len(taskService.StartedTasks)).To(Equal(2))

				{ // Check that first task propagates its resume error
					actionRunner.ResumeErr = errors.New("fake-resume-error-1")
					value, err := taskService.StartedTasks["fake-task-id-1"].Func()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-resume-error-1"))
					Expect(value).To(BeNil())
					Expect(actionRunner.ResumeAction).To(Equal(firstAction))
					Expect(string(actionRunner.ResumePayload)).To(Equal("fake-task-payload-1"))
				}

				{ // Check that second task propagates its resume error
					actionRunner.ResumeErr = errors.New("fake-resume-error-2")
					value, err := taskService.StartedTasks["fake-task-id-2"].Func()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-resume-error-2"))
					Expect(value).To(BeNil())
					Expect(actionRunner.ResumeAction).To(Equal(secondAction))
					Expect(string(actionRunner.ResumePayload)).To(Equal("fake-task-payload-2"))
				}
			})

			It("ignores actions that cannot be created and removes them from task manager", func() {
				actionFactory.RegisterActionErr("fake-action-1", errors.New("fake-action-error-1"))
				actionFactory.RegisterAction("fake-action-2", secondAction)

				dispatcher.ResumePreviouslyDispatchedTasks()
				Expect(len(taskService.StartedTasks)).To(Equal(1))

				{ // Check that first action is removed from task manager
					taskInfos, err := taskManager.GetInfos()
					Expect(err).ToNot(HaveOccurred())
					Expect(taskInfos).To(Equal([]boshtask.Info{
						boshtask.Info{
							TaskID:  "fake-task-id-2",
							Method:  "fake-action-2",
							Payload: []byte("fake-task-payload-2"),
						},
					}))
				}

				{ // Check that second task executes second action
					_, err := taskService.StartedTasks["fake-task-id-2"].Func()
					Expect(err).NotTo(HaveOccurred())
					Expect(actionRunner.ResumeAction).To(Equal(secondAction))
					Expect(string(actionRunner.ResumePayload)).To(Equal("fake-task-payload-2"))
				}
			})

			It("allows to cancel after resume", func() {
				actionFactory.RegisterAction("fake-action-1", firstAction)
				actionFactory.RegisterAction("fake-action-2", secondAction)

				dispatcher.ResumePreviouslyDispatchedTasks()

				err := taskService.StartedTasks["fake-task-id-1"].Cancel()
				Expect(err).ToNot(HaveOccurred())
				Expect(firstAction.Canceled).To(BeTrue())
				Expect(secondAction.Canceled).To(BeFalse())

				err = taskService.StartedTasks["fake-task-id-2"].Cancel()
				Expect(err).ToNot(HaveOccurred())
				Expect(secondAction.Canceled).To(BeTrue())
			})

			It("returns error from cancelling task when canceling resumed task fails", func() {
				actionFactory.RegisterAction("fake-action-1", firstAction)
				actionFactory.RegisterAction("fake-action-2", secondAction)

				dispatcher.ResumePreviouslyDispatchedTasks()

				firstAction.CancelErr = errors.New("fake-cancel-err-1")
				secondAction.CancelErr = errors.New("fake-cancel-err-2")

				err := taskService.StartedTasks["fake-task-id-1"].Cancel()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-cancel-err-1"))

				err = taskService.StartedTasks["fake-task-id-2"].Cancel()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-cancel-err-2"))
			})
		})
	})
}
