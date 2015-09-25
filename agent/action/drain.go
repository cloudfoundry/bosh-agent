package action

import (
	"errors"

	"fmt"
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
	if drainType == DrainTypeStatus {
		// status was used in the past when dynamic drain was implemented in director.
		// now that we implement it in the agent, we should never get a call for this type.
		return 0, bosherr.Error("Unexpected call with drain type 'status'")
	}

	a.logger.Debug(drainActionLogTag, "Running drain action with drain type %s", drainType)
	currentSpec, err := a.specService.Get()
	if err != nil {
		return 0, bosherr.WrapError(err, "Getting current spec")
	}

	a.logger.Debug(drainActionLogTag, "Unmonitoring")
	err = a.jobSupervisor.Unmonitor()
	if err != nil {
		return 0, bosherr.WrapError(err, "Unmonitoring services")
	}

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
	}

	drainScripts := a.findDrainScripts(currentSpec)

	a.logger.Debug(drainActionLogTag, "Will run %d drain scripts in parallel", len(drainScripts))

	resultChan := make(chan error)
	for _, drainScript := range drainScripts {
		drainScript := drainScript
		go func() { resultChan <- drainScript.Run(params) }()
	}

	var errors []error
	for i := 0; i < len(drainScripts); i++ {
		select {
		case err := <-resultChan:
			if err != nil {
				errors = append(errors, err)
			}
		}
	}
	return summarize(errors)
}

func (a DrainAction) findDrainScripts(currentSpec boshas.V1ApplySpec) []boshdrain.Script {
	var scripts []boshdrain.Script
	for _, job := range currentSpec.Jobs() {
		drainScript := a.drainScriptProvider.NewScript(job.BundleName())
		if drainScript.Exists() {
			a.logger.Debug(drainActionLogTag, "Found drain script in %s", job.BundleName())
			scripts = append(scripts, drainScript)
		}
	}
	return scripts
}

func summarize(errors []error) (int, error) {
	if len(errors) > 0 {
		var errorMsg string
		for _, err := range errors {
			errorMsg += err.Error() + "\n"
		}
		return 0, bosherr.Error(fmt.Sprintf("%d drain script(s) failed: %s", len(errors), errorMsg))
	}
	return 0, nil
}

func (a DrainAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a DrainAction) Cancel() error {
	return errors.New("not supported")
}
