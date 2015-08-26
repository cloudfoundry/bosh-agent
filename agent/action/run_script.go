package action

import (
	"errors"
	"github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	"github.com/cloudfoundry/bosh-agent/agent/scriptrunner"
	bosherr "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
)

type RunScriptAction struct {
	scriptProvider scriptrunner.JobScriptProvider
	specService    applyspec.V1Service
	logger         logger.Logger
}

func NewRunScript(
	scriptProvider scriptrunner.JobScriptProvider,
	specService applyspec.V1Service,
	logger logger.Logger,
) RunScriptAction {
	return RunScriptAction{
		scriptProvider: scriptProvider,
		specService:    specService,
		logger:         logger,
	}
}

func (a RunScriptAction) IsAsynchronous() bool {
	return true
}

func (a RunScriptAction) IsPersistent() bool {
	return false
}

func (a RunScriptAction) Run(scriptName string, options map[string]interface{}) (map[string]string, error) {
	result := map[string]string{}

	currentSpec, err := a.specService.Get()
	if err != nil {
		return result, bosherr.WrapError(err, "Getting current spec")
	}

	a.logger.Info("run-script-action", "Attempting to run '%s' scripts in %d jobs", scriptName, len(currentSpec.JobSpec.JobTemplateSpecs))

	scriptCount := 0
	failCount := 0

	for _, jobTemplate := range currentSpec.JobSpec.JobTemplateSpecs {
		script := a.scriptProvider.Get(jobTemplate.Name, scriptName)

		if script.Exists() {
			scriptCount++

			_, _, err := script.Run()
			if err != nil {
				// in an upcoming story, we log each job's output to separate files. TODO: record err in the log file
				result[jobTemplate.Name] = "failed"
				failCount++
			} else {
				result[jobTemplate.Name] = "executed"
			}
		}
	}

	if failCount > 0 {
		return result, bosherr.Errorf("%d of %d %s scripts failed. See logs for details.", failCount, scriptCount, scriptName)
	}
	return result, nil
}

func (a RunScriptAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a RunScriptAction) Cancel() error {
	return errors.New("not supported")
}
