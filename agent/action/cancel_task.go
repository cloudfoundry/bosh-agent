package action

import (
	"errors"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	boshtask "github.com/cloudfoundry/bosh-agent/v2/agent/task"
)

type CancelTaskAction struct {
	taskService boshtask.Service
}

func NewCancelTask(taskService boshtask.Service) (getTask CancelTaskAction) {
	getTask.taskService = taskService
	return
}

func (a CancelTaskAction) IsAsynchronous(_ ProtocolVersion) bool {
	return false
}

func (a CancelTaskAction) IsPersistent() bool {
	return false
}

func (a CancelTaskAction) IsLoggable() bool {
	return true
}

func (a CancelTaskAction) Run(taskID string) (string, error) {
	task, found := a.taskService.FindTaskWithID(taskID)
	if !found {
		return "", bosherr.Errorf("Task with id %s could not be found", taskID)
	}

	return "canceled", task.Cancel()
}

func (a CancelTaskAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a CancelTaskAction) Cancel() error {
	return errors.New("not supported")
}
