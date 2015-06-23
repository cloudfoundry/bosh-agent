package action

import (
	"errors"

	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshdrain "github.com/cloudfoundry/bosh-agent/agent/drain"
	bosherr "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	boshnotif "github.com/cloudfoundry/bosh-agent/notification"
)

const (
	drainActionLogTag = "Drain Action"
)

type DrainAction struct {
	drainScriptProvider boshdrain.ScriptProvider
	notifier            boshnotif.Notifier
	specService         boshas.V1Service
	jobSupervisor       boshjobsuper.JobSupervisor
	logger              boshlog.Logger
}

func NewDrain(
	notifier boshnotif.Notifier,
	specService boshas.V1Service,
	drainScriptProvider boshdrain.ScriptProvider,
	jobSupervisor boshjobsuper.JobSupervisor,
	logger boshlog.Logger,
) (drain DrainAction) {
	drain.notifier = notifier
	drain.specService = specService
	drain.drainScriptProvider = drainScriptProvider
	drain.jobSupervisor = jobSupervisor
	drain.logger = logger
	return
}

func (a DrainAction) IsAsynchronous() bool {
	return true
}

func (a DrainAction) IsPersistent() bool {
	return false
}

type DrainType string

const (
	DrainTypeUpdate   DrainType = "update"
	DrainTypeStatus   DrainType = "status"
	DrainTypeShutdown DrainType = "shutdown"
)

func (a DrainAction) Run(drainType DrainType, newSpecs ...boshas.V1ApplySpec) (int, error) {
	a.logger.Debug(drainActionLogTag, "Running drain action with drain type %s", drainType)
	currentSpec, err := a.specService.Get()
	if err != nil {
		return 0, bosherr.WrapError(err, "Getting current spec")
	}

	if len(currentSpec.JobSpec.Template) == 0 {
		if drainType == DrainTypeStatus {
			return 0, bosherr.Error("Check Status on Drain action requires job spec")
		}
		return 0, nil
	}

	a.logger.Debug(drainActionLogTag, "Unmonitoring")
	err = a.jobSupervisor.Unmonitor()
	if err != nil {
		return 0, bosherr.WrapError(err, "Unmonitoring services")
	}

	drainScript := a.drainScriptProvider.NewScript(currentSpec.JobSpec.Template)

	var newSpec *boshas.V1ApplySpec
	var params boshdrain.ScriptParams

	if len(newSpecs) > 0 {
		newSpec = &newSpecs[0]
	}

	switch drainType {
	case DrainTypeUpdate:
		if newSpec == nil {
			return 0, bosherr.Error("Drain update requires new spec")
		}

		params = boshdrain.NewUpdateParams(currentSpec, *newSpec)

	case DrainTypeShutdown:
		err = a.notifier.NotifyShutdown()
		if err != nil {
			return 0, bosherr.WrapError(err, "Notifying shutdown")
		}

		params = boshdrain.NewShutdownParams(currentSpec, newSpec)

	case DrainTypeStatus:
		params = boshdrain.NewStatusParams(currentSpec, newSpec)
	}

	if !drainScript.Exists() {
		if drainType == DrainTypeStatus {
			return 0, bosherr.Error("Check Status on Drain action requires a valid drain script")
		}
		return 0, nil
	}

	value, err := drainScript.Run(params)
	if err != nil {
		return 0, bosherr.WrapError(err, "Running Drain Script")
	}

	return value, nil
}

func (a DrainAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a DrainAction) Cancel() error {
	return errors.New("not supported")
}
