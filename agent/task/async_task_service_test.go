package task_test

import (
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakeuuid "github.com/cloudfoundry/bosh-utils/uuid/fakes"

	. "github.com/cloudfoundry/bosh-agent/v2/agent/task"
)

func init() { //nolint:funlen,gochecknoinits
	Describe("asyncTaskService", func() {
		var (
			uuidGen *fakeuuid.FakeGenerator
			service Service
		)

		BeforeEach(func() {
			uuidGen = &fakeuuid.FakeGenerator{}
			service = NewAsyncTaskService(uuidGen, boshlog.NewLogger(boshlog.LevelNone))
		})

		Describe("StartTask", func() {
			startAndWaitForTaskCompletion := func(task Task) Task {
				service.StartTask(task)
				for task.State == StateRunning {
					time.Sleep(time.Nanosecond)
					task, _ = service.FindTaskWithID(task.ID)
				}
				return task
			}

			It("sets return value on a successful task", func() {
				runFunc := func() (interface{}, error) { return 123, nil }

				task, err := service.CreateTask(runFunc, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				task = startAndWaitForTaskCompletion(task)
				Expect(task.State).To(BeEquivalentTo(StateDone))
				Expect(task.Value).To(Equal(123))
				Expect(task.Error).To(BeNil())
			})

			It("sets task error on a failing task", func() {
				err := errors.New("fake-error")
				runFunc := func() (interface{}, error) { return nil, err }

				task, createErr := service.CreateTask(runFunc, nil, nil)
				Expect(createErr).ToNot(HaveOccurred())

				task = startAndWaitForTaskCompletion(task)
				Expect(task.State).To(BeEquivalentTo(StateFailed))
				Expect(task.Value).To(BeNil())
				Expect(task.Error).To(Equal(err))
			})

			It("sets task Func, CancelFunc and EndFunc to nil on a successful task", func() {
				runFunc := func() (interface{}, error) { return nil, nil }
				cancelFunc := func(_ Task) error { return nil }
				endFunc := func(_ Task) {}

				task, createErr := service.CreateTask(runFunc, cancelFunc, endFunc)
				Expect(createErr).ToNot(HaveOccurred())

				task = startAndWaitForTaskCompletion(task)
				Expect(task.Func).To(BeNil())
				Expect(task.CancelFunc).To(BeNil())
				Expect(task.EndFunc).To(BeNil())
			})

			It("sets task Func, CancelFunc and EndFunc to nil on a failing task", func() {
				runFunc := func() (interface{}, error) { return nil, errors.New("fake-error") }
				cancelFunc := func(_ Task) error { return nil }
				endFunc := func(_ Task) {}

				task, createErr := service.CreateTask(runFunc, cancelFunc, endFunc)
				Expect(createErr).ToNot(HaveOccurred())

				task = startAndWaitForTaskCompletion(task)
				Expect(task.Func).To(BeNil())
				Expect(task.CancelFunc).To(BeNil())
				Expect(task.EndFunc).To(BeNil())
			})

			Describe("CreateTask", func() {
				It("can run task created with CreateTask which does not have end func", func() {
					ranFunc := false
					runFunc := func() (interface{}, error) { ranFunc = true; return nil, nil }

					task, err := service.CreateTask(runFunc, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					startAndWaitForTaskCompletion(task)
					Expect(ranFunc).To(BeTrue())
				})

				It("can run task created with CreateTask which has end func", func() {
					ranFunc := false
					runFunc := func() (interface{}, error) { ranFunc = true; return nil, nil }

					ranEndFunc := false
					endFunc := func(Task) { ranEndFunc = true }

					task, err := service.CreateTask(runFunc, nil, endFunc)
					Expect(err).ToNot(HaveOccurred())

					startAndWaitForTaskCompletion(task)
					Expect(ranFunc).To(BeTrue())
					Expect(ranEndFunc).To(BeTrue())
				})

				It("returns an error if generate uuid fails", func() {
					uuidGen.GenerateError = errors.New("fake-generate-uuid-error")
					_, err := service.CreateTask(nil, nil, nil)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-generate-uuid-error"))
				})
			})

			Describe("CreateTaskWithID", func() {
				It("can run task created with CreateTaskWithID which does not have end func", func() {
					ranFunc := false
					runFunc := func() (interface{}, error) { ranFunc = true; return nil, nil }

					task := service.CreateTaskWithID("fake-task-id", runFunc, nil, nil)

					startAndWaitForTaskCompletion(task)
					Expect(ranFunc).To(BeTrue())
				})

				It("can run task created with CreateTaskWithID which has end func", func() {
					ranFunc := false
					runFunc := func() (interface{}, error) { ranFunc = true; return nil, nil }

					ranEndFunc := false
					endFunc := func(Task) { ranEndFunc = true }

					task := service.CreateTaskWithID("fake-task-id", runFunc, nil, endFunc)

					startAndWaitForTaskCompletion(task)
					Expect(ranFunc).To(BeTrue())
					Expect(ranEndFunc).To(BeTrue())
				})
			})

			It("can process many tasks simultaneously", func() {
				taskFunc := func() (interface{}, error) {
					time.Sleep(10 * time.Millisecond)
					return nil, nil
				}

				ids := []string{}
				for id := 1; id < 200; id++ {
					idStr := fmt.Sprintf("%d", id)
					uuidGen.GeneratedUUID = idStr
					ids = append(ids, idStr)

					task, err := service.CreateTask(taskFunc, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					go service.StartTask(task)
				}

				for {
					allDone := true
					for _, id := range ids {
						task, _ := service.FindTaskWithID(id)
						if task.State != StateDone {
							allDone = false
							break
						}
					}

					if allDone {
						break
					}
					time.Sleep(200 * time.Millisecond)
				}
			})

			It("will not block if another task is already started", func(ctx SpecContext) {
				taskChannel := make(chan bool)
				task1Func := func() (interface{}, error) {
					<-taskChannel
					return nil, nil
				}
				task1, _ := service.CreateTask(task1Func, nil, nil) //nolint:errcheck
				service.StartTask(task1)
				task2Func := func() (interface{}, error) {
					return nil, nil
				}
				task2, _ := service.CreateTask(task2Func, nil, nil) //nolint:errcheck
				Eventually(func() bool {
					service.StartTask(task2)
					return true
				}).WithContext(ctx).Should(BeTrue())

				taskChannel <- true
			}, SpecTimeout(time.Second*5))
		})

		Describe("CreateTask", func() {
			It("creates a task with auto-assigned id", func() {
				uuidGen.GeneratedUUID = "fake-uuid"

				runFuncCalled := false
				runFunc := func() (interface{}, error) {
					runFuncCalled = true
					return nil, nil
				}

				cancelFuncCalled := false
				cancelFunc := func(_ Task) error {
					cancelFuncCalled = true
					return nil
				}

				endFuncCalled := false
				endFunc := func(_ Task) {
					endFuncCalled = true
				}

				task, err := service.CreateTask(runFunc, cancelFunc, endFunc)
				Expect(err).ToNot(HaveOccurred())
				Expect(task.ID).To(Equal("fake-uuid"))
				Expect(task.State).To(Equal(StateRunning))

				_, err = task.Func()
				Expect(err).NotTo(HaveOccurred())
				Expect(runFuncCalled).To(BeTrue())

				err = task.CancelFunc(task)
				Expect(err).NotTo(HaveOccurred())
				Expect(cancelFuncCalled).To(BeTrue())

				task.EndFunc(task)
				Expect(endFuncCalled).To(BeTrue())
			})
		})

		Describe("CreateTaskWithID", func() {
			It("creates a task with given id", func() {
				runFuncCalled := false
				runFunc := func() (interface{}, error) {
					runFuncCalled = true
					return nil, nil
				}

				cancelFuncCalled := false
				cancelFunc := func(_ Task) error {
					cancelFuncCalled = true
					return nil
				}

				endFuncCalled := false
				endFunc := func(_ Task) {
					endFuncCalled = true
				}

				task := service.CreateTaskWithID("fake-task-id", runFunc, cancelFunc, endFunc)
				Expect(task.ID).To(Equal("fake-task-id"))
				Expect(task.State).To(Equal(StateRunning))

				_, err := task.Func()
				Expect(err).NotTo(HaveOccurred())
				Expect(runFuncCalled).To(BeTrue())

				err = task.CancelFunc(task)
				Expect(err).NotTo(HaveOccurred())
				Expect(cancelFuncCalled).To(BeTrue())

				task.EndFunc(task)
				Expect(endFuncCalled).To(BeTrue())
			})
		})
	})
}
