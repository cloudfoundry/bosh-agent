package action

import (
	"errors"
	"time"

	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshscript "github.com/cloudfoundry/bosh-agent/agent/script"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const runErrandActionLogTag = "runErrandAction"

type RunErrandAction struct {
	jobScriptProvider boshscript.JobScriptProvider
	specService       boshas.V1Service
	jobsDir           string
	logger            boshlog.Logger

	cancelCh chan struct{}
}

func NewRunErrand(
	jobScriptProvider boshscript.JobScriptProvider,
	specService boshas.V1Service,
	jobsDir string,
	logger boshlog.Logger,
) RunErrandAction {
	return RunErrandAction{
		jobScriptProvider: jobScriptProvider,
		specService:       specService,
		jobsDir:           jobsDir,
		logger:            logger,

		// Initialize channel in a constructor to avoid race
		// between initializing in Run()/Cancel()
		cancelCh: make(chan struct{}, 1),
	}
}

func (a RunErrandAction) IsAsynchronous() bool {
	return true
}

func (a RunErrandAction) IsPersistent() bool {
	return false
}

type ErrandResult struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitStatus int    `json:"exit_code"`
}

func (a RunErrandAction) Run() (ErrandResult, error) {
	currentSpec, err := a.specService.Get()
	if err != nil {
		return ErrandResult{}, bosherr.WrapError(err, "Getting current spec")
	}

	if len(currentSpec.JobSpec.Template) == 0 {
		return ErrandResult{}, bosherr.Error("At least one job template is required to run an errand")
	}

	errandScript := a.jobScriptProvider.NewScript(currentSpec.JobSpec.Template, "run")

	process, err := errandScript.RunAsync()
	if err != nil {
		return ErrandResult{}, bosherr.WrapError(err, "Running errand script")
	}

	var result boshsys.Result

	// Can only wait once on a process but cancelling can happen multiple times
	for processExitedCh := process.Wait(); processExitedCh != nil; {
		select {
		case result = <-processExitedCh:
			processExitedCh = nil
		case <-a.cancelCh:
			// Ignore possible TerminateNicely error since we cannot return it
			err := process.TerminateNicely(10 * time.Second)
			if err != nil {
				a.logger.Error(runErrandActionLogTag, "Failed to terminate %s", err.Error())
			}
		}
	}

	if result.Error != nil && result.ExitStatus == -1 {
		return ErrandResult{}, bosherr.WrapError(result.Error, "Running errand script")
	}

	return ErrandResult{
		Stdout:     "Truncated stdout here",
		Stderr:     "Truncated stderr here",
		ExitStatus: result.ExitStatus,
	}, nil
}

func (a RunErrandAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

// Cancelling rules:
// 1. Cancel action MUST take constant time even if another cancel is pending/running
// 2. Cancel action DOES NOT have to cancel if another cancel is pending/running
// 3. Cancelling errand before it starts should cancel errand when it runs
//    - possible optimization - do not even start errand
// (e.g. send 5 cancels, 1 is actually doing cancelling, other 4 exit immediately)

// Cancel satisfies above rules though it never returns any error
func (a RunErrandAction) Cancel() error {
	select {
	case a.cancelCh <- struct{}{}:
		// Always return no error since we cannot wait until
		// errand runs in the future and potentially fails to cancel

	default:
		// Cancel action is already queued up
	}
	return nil
}
