package jobsupervisor

import (
	"encoding/json"

	boshalert "github.com/cloudfoundry/bosh-agent/agent/alert"
	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	bosherror "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/errors"
)

type dummyNatsJobSupervisor struct {
	mbusHandler       boshhandler.Handler
	status            string
	processes         []Process
	jobFailureHandler JobFailureHandler
}

func NewDummyNatsJobSupervisor(mbusHandler boshhandler.Handler) JobSupervisor {
	return &dummyNatsJobSupervisor{
		mbusHandler: mbusHandler,
		status:      "running",
		processes:   []Process{},
	}
}

func (d *dummyNatsJobSupervisor) Reload() error {
	return nil
}

func (d *dummyNatsJobSupervisor) AddJob(jobName string, jobIndex int, configPath string) error {
	return nil
}

func (d *dummyNatsJobSupervisor) Start() error {
	if d.status == "fail_task" {
		return bosherror.Error("fake-task-fail-error")
	}
	if d.status != "failing" {
		d.status = "running"
	}
	return nil
}

func (d *dummyNatsJobSupervisor) Stop() error {
	if d.status != "failing" && d.status != "fail_task" {
		d.status = "stopped"
	}
	return nil
}

func (d *dummyNatsJobSupervisor) Unmonitor() error {
	return nil
}

func (d *dummyNatsJobSupervisor) RemoveAllJobs() error {
	return nil
}

func (d *dummyNatsJobSupervisor) Status() string {
	return d.status
}

func (d *dummyNatsJobSupervisor) Processes() ([]Process, error) {
	return d.processes, nil
}

func (d *dummyNatsJobSupervisor) MonitorJobFailures(handler JobFailureHandler) error {
	d.jobFailureHandler = handler

	d.mbusHandler.RegisterAdditionalFunc(d.statusHandler)

	return nil
}

func (d *dummyNatsJobSupervisor) statusHandler(req boshhandler.Request) boshhandler.Response {
	switch req.Method {
	case "set_dummy_status":
		// Do not unmarshal message until determining its method
		var body map[string]string

		err := json.Unmarshal(req.GetPayload(), &body)
		if err != nil {
			return boshhandler.NewExceptionResponse(err)
		}

		d.status = body["status"]

		if d.status == "failing" && d.jobFailureHandler != nil {
			_ = d.jobFailureHandler(boshalert.MonitAlert{
				ID:          "fake-monit-alert",
				Service:     "fake-monit-service",
				Event:       "failing",
				Action:      "start",
				Date:        "Sun, 22 May 2011 20:07:41 +0500",
				Description: "fake-monit-description",
			})
		}

		return boshhandler.NewValueResponse("ok")

	case "set_task_fail":
		// Do not unmarshal message until determining its method
		var body map[string]string

		err := json.Unmarshal(req.GetPayload(), &body)
		if err != nil {
			return boshhandler.NewExceptionResponse(err)
		}

		d.status = body["status"]

		return nil
	default:
		return nil
	}
}
