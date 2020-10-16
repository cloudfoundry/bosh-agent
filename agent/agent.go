package agent

import (
	"time"

	"code.cloudfoundry.org/clock"

	boshalert "github.com/cloudfoundry/bosh-agent/agent/alert"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
)

const (
	agentLogTag         = "agent"
	heartbeatMaxRetries = 60
)

var (
	HeartbeatRetryInterval = 1 * time.Second
)

//go:generate counterfeiter . StartManager

type StartManager interface {
	CanStart() bool
	RegisterStart() error
}

type Agent struct {
	logger            boshlog.Logger
	mbusHandler       boshhandler.Handler
	platform          boshplatform.Platform
	actionDispatcher  ActionDispatcher
	heartbeatInterval time.Duration
	jobSupervisor     boshjobsuper.JobSupervisor
	specService       boshas.V1Service
	settingsService   boshsettings.Service
	uuidGenerator     boshuuid.Generator
	timeService       clock.Clock
	startManager      StartManager
}

func New(
	logger boshlog.Logger,
	mbusHandler boshhandler.Handler,
	platform boshplatform.Platform,
	actionDispatcher ActionDispatcher,
	jobSupervisor boshjobsuper.JobSupervisor,
	specService boshas.V1Service,
	heartbeatInterval time.Duration,
	settingsService boshsettings.Service,
	uuidGenerator boshuuid.Generator,
	timeService clock.Clock,
	startManager StartManager,
) Agent {
	return Agent{
		logger:            logger,
		mbusHandler:       mbusHandler,
		platform:          platform,
		actionDispatcher:  actionDispatcher,
		heartbeatInterval: heartbeatInterval,
		jobSupervisor:     jobSupervisor,
		specService:       specService,
		settingsService:   settingsService,
		uuidGenerator:     uuidGenerator,
		timeService:       timeService,
		startManager:      startManager,
	}
}

func (a Agent) Run() error {
	if !a.startManager.CanStart() {
		return bosherr.Error("Refusing to boot")
	}
	if err := a.startManager.RegisterStart(); err != nil {
		return bosherr.WrapError(err, "Registering start")
	}

	errCh := make(chan error, 1)

	a.actionDispatcher.ResumePreviouslyDispatchedTasks()

	go a.subscribeActionDispatcher(errCh)

	go a.generateHeartbeats(errCh)

	go func() {
		err := a.jobSupervisor.MonitorJobFailures(a.handleJobFailure(errCh))
		if err != nil {
			errCh <- err
		}
	}()

	return <-errCh
}

func (a Agent) subscribeActionDispatcher(errCh chan error) {
	defer a.logger.HandlePanic("Agent Message Bus Handler")

	err := a.mbusHandler.Run(a.actionDispatcher.Dispatch)
	if err != nil {
		err = bosherr.WrapError(err, "Message Bus Handler")
	}

	errCh <- err
}

func (a Agent) generateHeartbeats(errCh chan error) {
	a.logger.Debug(agentLogTag, "Generating heartbeat")
	defer a.logger.HandlePanic("Agent Generate Heartbeats")

	// Send initial heartbeat
	a.sendAndRecordHeartbeat(errCh, false)

	tickChan := time.Tick(a.heartbeatInterval)

	for {
		select {
		case <-tickChan:
			a.sendAndRecordHeartbeat(errCh, true)
		}
	}
}

func (a Agent) sendAndRecordHeartbeat(errCh chan error, retry bool) {
	status := a.jobSupervisor.Status()
	heartbeat, err := a.getHeartbeat(status)
	if err != nil {
		err = bosherr.WrapError(err, "Building heartbeat")
		errCh <- err
		return
	}
	a.jobSupervisor.HealthRecorder(status)

	heartbeatRetryable := boshretry.NewRetryable(func() (bool, error) {
		a.logger.Info(agentLogTag, "Attempting to send Heartbeat")

		err = a.mbusHandler.Send(boshhandler.HealthMonitor, boshhandler.Heartbeat, heartbeat)
		if err != nil {
			return true, bosherr.WrapError(err, "Sending Heartbeat")
		}
		return false, nil
	})

	retries := 1
	if retry {
		retries = heartbeatMaxRetries
	}

	attemptRetryStrategy := boshretry.NewAttemptRetryStrategy(
		retries,
		HeartbeatRetryInterval,
		heartbeatRetryable,
		a.logger,
	)
	err = attemptRetryStrategy.Try()

	if err != nil {
		errCh <- err
	}
}

func (a Agent) getHeartbeat(status string) (Heartbeat, error) {
	a.logger.Debug(agentLogTag, "Building heartbeat")
	vitalsService := a.platform.GetVitalsService()

	vitals, err := vitalsService.Get()
	if err != nil {
		return Heartbeat{}, bosherr.WrapError(err, "Getting job vitals")
	}

	spec, err := a.specService.Get()
	if err != nil {
		return Heartbeat{}, bosherr.WrapError(err, "Getting job spec")
	}

	hb := Heartbeat{
		Deployment: spec.Deployment,
		Job:        spec.JobSpec.Name,
		Index:      spec.Index,
		JobState:   status,
		Vitals:     vitals,
		NodeID:     spec.NodeID,
	}

	return hb, nil
}

func (a Agent) handleJobFailure(errCh chan error) boshjobsuper.JobFailureHandler {
	return func(monitAlert boshalert.MonitAlert) error {
		alertAdapter := boshalert.NewMonitAdapter(monitAlert, a.settingsService, a.timeService)
		if alertAdapter.IsIgnorable() {
			a.logger.Debug(agentLogTag, "Ignored monit event: ", monitAlert.Event)
			return nil
		}

		severity, found := alertAdapter.Severity()
		if !found {
			a.logger.Error(agentLogTag, "Unknown monit event name `%s', using default severity %d", monitAlert.Event, severity)
		}

		alert, err := alertAdapter.Alert()
		if err != nil {
			errCh <- bosherr.WrapError(err, "Adapting monit alert")
		}

		err = a.mbusHandler.Send(boshhandler.HealthMonitor, boshhandler.Alert, alert)
		if err != nil {
			errCh <- bosherr.WrapError(err, "Sending monit alert")
		}

		return nil
	}
}
