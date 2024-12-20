package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshaction "github.com/cloudfoundry/bosh-agent/v2/agent/action"
	boshtask "github.com/cloudfoundry/bosh-agent/v2/agent/task"
	faketask "github.com/cloudfoundry/bosh-agent/v2/agent/task/fakes"
)

var _ = Describe("CancelTaskAction", func() {
	var (
		taskService *faketask.FakeService
		action      boshaction.CancelTaskAction
	)

	BeforeEach(func() {
		taskService = faketask.NewFakeService()
		action = boshaction.NewCancelTask(taskService)
	})

	AssertActionIsNotAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotCancelable(action)
	AssertActionIsNotResumable(action)

	It("cancels task if task is found", func() {
		cancelCalled := false
		cancelFunc := func(_ boshtask.Task) error { cancelCalled = true; return nil }

		taskService.StartedTasks["fake-task-id"] = boshtask.Task{
			ID:         "fake-task-id",
			State:      boshtask.StateRunning,
			CancelFunc: cancelFunc,
		}

		value, err := action.Run("fake-task-id")
		Expect(err).ToNot(HaveOccurred())
		Expect(value).To(Equal("canceled")) // 1 l

		Expect(cancelCalled).To(BeTrue())
	})

	It("returns error when canceling task fails", func() {
		cancelFunc := func(_ boshtask.Task) error { return errors.New("fake-cancel-err") }

		taskService.StartedTasks["fake-task-id"] = boshtask.Task{
			ID:         "fake-task-id",
			State:      boshtask.StateRunning,
			CancelFunc: cancelFunc,
		}

		_, err := action.Run("fake-task-id")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("fake-cancel-err"))
	})

	It("returns error when task is not found", func() {
		taskService.StartedTasks = map[string]boshtask.Task{}

		_, err := action.Run("fake-task-id")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Task with id fake-task-id could not be found"))
	})
})
