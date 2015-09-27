package action

import (
	"errors"

	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshdrain "github.com/cloudfoundry/bosh-agent/agent/script/drain"
	bosherr "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	boshnotif "github.com/cloudfoundry/bosh-agent/notification"
)

type DrainAction struct {
	drainScriptProvider boshdrain.ScriptProvider
	notifier            boshnotif.Notifier
	specService         boshas.V1Service
	jobSupervisor       boshjobsuper.JobSupervisor

	logTag string
	logger boshlog.Logger
}

type DrainType string

const (
	DrainTypeUpdate   DrainType = "update"
	DrainTypeStatus   DrainType = "status"
	DrainTypeShutdown DrainType = "shutdown"
)

func NewDrain(
	notifier boshnotif.Notifier,
	specService boshas.V1Service,
	drainScriptProvider boshdrain.ScriptProvider,
	jobSupervisor boshjobsuper.JobSupervisor,
	logger boshlog.Logger,
) DrainAction {
	return DrainAction{
		notifier:            notifier,
		specService:         specService,
		drainScriptProvider: drainScriptProvider,
		jobSupervisor:       jobSupervisor,

		logTag: "Drain Action",
		logger: logger,
	}
}

func (a DrainAction) IsAsynchronous() bool {
	return true
}

func (a DrainAction) IsPersistent() bool {
	return false
}

func (a DrainAction) Run(drainType DrainType, newSpecs ...boshas.V1ApplySpec) (int, error) {
	currentSpec, err := a.specService.Get()
	if err != nil {
		return 0, bosherr.WrapError(err, "Getting current spec")
	}

	params, err := a.determineParams(drainType, currentSpec, newSpecs)
	if err != nil {
		return 0, err
	}

	a.logger.Debug(a.logTag, "Unmonitoring")

	err = a.jobSupervisor.Unmonitor()
	if err != nil {
		return 0, bosherr.WrapError(err, "Unmonitoring services")
	}

	drainScripts := a.findDrainScripts(currentSpec, params)

	a.logger.Debug(a.logTag, "Will run '%d' drain scripts in parallel", len(drainScripts))

	resultChan := make(chan error)

	for _, drainScript := range drainScripts {
		drainScript := drainScript
		go func() { resultChan <- drainScript.Run() }()
	}

	var errs []error

	for i := 0; i < len(drainScripts); i++ {
		select {
		case err := <-resultChan:
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	return a.summarizeErrs(errs)
}

func (a DrainAction) determineParams(drainType DrainType, currentSpec boshas.V1ApplySpec, newSpecs []boshas.V1ApplySpec) (boshdrain.ScriptParams, error) {
	var newSpec *boshas.V1ApplySpec
	var params boshdrain.ScriptParams

	if len(newSpecs) > 0 {
		newSpec = &newSpecs[0]
	}

	switch drainType {
	case DrainTypeStatus:
		// Status was used in the past when dynamic drain was implemented in the Director.
		// Now that we implement it in the agent, we should never get a call for this type.
		return params, bosherr.Error("Unexpected call with drain type 'status'")

	case DrainTypeUpdate:
		if newSpec == nil {
			return params, bosherr.Error("Drain update requires new spec")
		}

		params = boshdrain.NewUpdateParams(currentSpec, *newSpec)

	case DrainTypeShutdown:
		err := a.notifier.NotifyShutdown()
		if err != nil {
			return params, bosherr.WrapError(err, "Notifying shutdown")
		}

		params = boshdrain.NewShutdownParams(currentSpec, newSpec)
	}

	return params, nil
}

func (a DrainAction) findDrainScripts(currentSpec boshas.V1ApplySpec, params boshdrain.ScriptParams) []boshdrain.Script {
	var scripts []boshdrain.Script

	for _, job := range currentSpec.Jobs() {
		script := a.drainScriptProvider.NewScript(job.BundleName(), params)
		if script.Exists() {
			a.logger.Debug(a.logTag, "Found drain script in job '%s'", job.BundleName())
			scripts = append(scripts, script)
		} else {
			a.logger.Debug(a.logTag, "Did not find drain script in job '%s'", job.BundleName())
		}
	}

	return scripts
}

func (a DrainAction) summarizeErrs(errs []error) (int, error) {
	if len(errs) > 0 {
		var errMsg string

		for _, err := range errs {
			errMsg += err.Error() + "\n"
		}

		return 0, bosherr.Errorf("'%d' drain script(s) failed: %s", len(errs), errMsg)
	}

	return 0, nil
}

func (a DrainAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a DrainAction) Cancel() error {
	return errors.New("not supported")
}
