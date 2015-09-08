package action

import (
	"errors"
	"github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	"github.com/cloudfoundry/bosh-agent/agent/scriptrunner"
	bosherr "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	"strings"
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

	errorChan := make(chan scriptrunner.RunScriptResult)
	doneChan := make(chan scriptrunner.RunScriptResult)

	for _, jobTemplate := range currentSpec.JobSpec.JobTemplateSpecs {
		script := a.scriptProvider.Get(jobTemplate.Name, scriptName)
		if script.Exists() {
			scriptCount++
			go script.Run(errorChan, doneChan)
		}
	}

	var failedScripts []string
	var passedScripts []string

	for i := 0; i < scriptCount; i++ {
		select {
		case failedScript := <-errorChan:
			result[failedScript.JobName] = "failed"
			failedScripts = append(failedScripts, failedScript.JobName)
			a.logger.Info("run-script-action", "'%s' script has failed", failedScript)
		case passedScript := <-doneChan:
			result[passedScript.JobName] = "executed"
			passedScripts = append(passedScripts, passedScript.JobName)
			a.logger.Info("run-script-action", "'%s' script has passed", passedScript)
		}
	}

	if len(failedScripts) > 0 {
		msg := "Failed Jobs: " + strings.Join(failedScripts, ", ")
		if len(passedScripts) > 0 {
			msg += ". Successful Jobs: " + strings.Join(passedScripts, ", ")
		}
		return result, bosherr.Errorf("%d of %d %s scripts failed. %s.", len(failedScripts), scriptCount, scriptName, msg)
	}
	return result, nil
}

func (a RunScriptAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a RunScriptAction) Cancel() error {
	return errors.New("not supported")
}
