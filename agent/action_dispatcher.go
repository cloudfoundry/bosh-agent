package agent

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	boshaction "github.com/cloudfoundry/bosh-agent/v2/agent/action"
	boshtask "github.com/cloudfoundry/bosh-agent/v2/agent/task"
	boshhandler "github.com/cloudfoundry/bosh-agent/v2/handler"
)

const actionDispatcherLogTag = "Action Dispatcher"

type ActionDispatcher interface {
	ResumePreviouslyDispatchedTasks()
	Dispatch(req boshhandler.Request) (resp boshhandler.Response)
}

type concreteActionDispatcher struct {
	logger        boshlog.Logger
	taskService   boshtask.Service
	taskManager   boshtask.Manager
	actionFactory boshaction.Factory
	actionRunner  boshaction.Runner
}

func NewActionDispatcher(
	logger boshlog.Logger,
	taskService boshtask.Service,
	taskManager boshtask.Manager,
	actionFactory boshaction.Factory,
	actionRunner boshaction.Runner,
) (dispatcher ActionDispatcher) {
	return concreteActionDispatcher{
		logger:        logger,
		taskService:   taskService,
		taskManager:   taskManager,
		actionFactory: actionFactory,
		actionRunner:  actionRunner,
	}
}

func (dispatcher concreteActionDispatcher) ResumePreviouslyDispatchedTasks() {
	taskInfos, err := dispatcher.taskManager.GetInfos()
	if err != nil {
		// Ignore failure of resuming tasks because there is nothing we can do.
		// API consumers will encounter unknown task id error when they request get_task.
		// Other option is to return an error which will cause agent to restart again
		// which does not help API consumer to determine that agent cannot continue tasks.
		dispatcher.logger.Error(actionDispatcherLogTag, err.Error())
		return
	}

	for _, taskInfo := range taskInfos {
		action, err := dispatcher.actionFactory.Create(taskInfo.Method)
		if err != nil {
			dispatcher.logger.Error(actionDispatcherLogTag, "Unknown action %s", taskInfo.Method)
			if removeErr := dispatcher.taskManager.RemoveInfo(taskInfo.TaskID); removeErr != nil {
				dispatcher.logger.Warn(actionDispatcherLogTag, "Failed to remove task info: %s", removeErr.Error())
			}
			continue
		}

		taskID := taskInfo.TaskID
		payload := taskInfo.Payload

		task := dispatcher.taskService.CreateTaskWithID(
			taskID,
			func() (interface{}, error) { return dispatcher.actionRunner.Resume(action, payload) },
			func(_ boshtask.Task) error { return action.Cancel() },
			dispatcher.removeInfo,
		)

		dispatcher.taskService.StartTask(task)
	}
}

func (dispatcher concreteActionDispatcher) Dispatch(req boshhandler.Request) boshhandler.Response {
	action, err := dispatcher.actionFactory.Create(req.Method)
	if err != nil {
		dispatcher.logger.Error(actionDispatcherLogTag, "Unknown action %s", req.Method)
		return boshhandler.NewExceptionResponse(bosherr.Errorf("unknown message %s", req.Method))
	}

	dispatcher.logger.Info(actionDispatcherLogTag, "Received request with action %s", req.Method)
	if action.IsLoggable() {
		dispatcher.logger.DebugWithDetails(actionDispatcherLogTag, "Payload", req.Payload)
	}

	if action.IsAsynchronous(boshaction.ProtocolVersion(req.ProtocolVersion)) {
		return dispatcher.dispatchAsynchronousAction(action, req)
	}

	return dispatcher.dispatchSynchronousAction(action, req)
}

func (dispatcher concreteActionDispatcher) dispatchAsynchronousAction(
	action boshaction.Action,
	req boshhandler.Request,
) boshhandler.Response {
	dispatcher.logger.Info(actionDispatcherLogTag, "Running async action %s", req.Method)

	var task boshtask.Task
	var err error

	runTask := func() (interface{}, error) {
		return dispatcher.actionRunner.Run(action, req.GetPayload(), boshaction.ProtocolVersion(req.ProtocolVersion))
	}

	cancelTask := func(_ boshtask.Task) error { return action.Cancel() }

	// Certain long-running tasks (e.g. configure_networks) must be resumed
	// after agent restart so that API consumers do not need to know
	// if agent is restarted midway through the task.
	if action.IsPersistent() {
		dispatcher.logger.Info(actionDispatcherLogTag, "Running persistent action %s", req.Method)
		task, err = dispatcher.taskService.CreateTask(runTask, cancelTask, dispatcher.removeInfo)
		if err != nil {
			err = bosherr.WrapErrorf(err, "Create Task Failed %s", req.Method)
			dispatcher.logger.Error(actionDispatcherLogTag, err.Error())
			return boshhandler.NewExceptionResponse(err)
		}

		taskInfo := boshtask.Info{
			TaskID:  task.ID,
			Method:  req.Method,
			Payload: req.GetPayload(),
		}

		err = dispatcher.taskManager.AddInfo(taskInfo)
		if err != nil {
			err = bosherr.WrapErrorf(err, "Action Failed %s", req.Method)
			dispatcher.logger.Error(actionDispatcherLogTag, err.Error())
			return boshhandler.NewExceptionResponse(err)
		}
	} else {
		task, err = dispatcher.taskService.CreateTask(runTask, cancelTask, nil)
		if err != nil {
			err = bosherr.WrapErrorf(err, "Create Task Failed %s", req.Method)
			dispatcher.logger.Error(actionDispatcherLogTag, err.Error())
			return boshhandler.NewExceptionResponse(err)
		}
	}

	dispatcher.taskService.StartTask(task)

	return boshhandler.NewValueResponse(boshtask.StateValue{
		AgentTaskID: task.ID,
		State:       task.State,
	})
}

func (dispatcher concreteActionDispatcher) dispatchSynchronousAction(
	action boshaction.Action,
	req boshhandler.Request,
) boshhandler.Response {
	dispatcher.logger.Info(actionDispatcherLogTag, "Running sync action %s", req.Method)

	value, err := dispatcher.actionRunner.Run(action, req.GetPayload(), boshaction.ProtocolVersion(req.ProtocolVersion))
	if err != nil {
		err = bosherr.WrapErrorf(err, "Action Failed %s", req.Method)
		dispatcher.logger.Error(actionDispatcherLogTag, err.Error())
		return boshhandler.NewExceptionResponse(err)
	}

	return boshhandler.NewValueResponse(value)
}

func (dispatcher concreteActionDispatcher) removeInfo(task boshtask.Task) {
	err := dispatcher.taskManager.RemoveInfo(task.ID)
	if err != nil {
		// There is not much we can do about failing to write state of a finished task.
		// On next agent restart, task will be Resume()d again so it must be idempotent.
		dispatcher.logger.Error(actionDispatcherLogTag, err.Error())
	}
}
