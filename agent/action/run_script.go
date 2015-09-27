package action

import (
	"errors"
	"strings"

	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	"github.com/cloudfoundry/bosh-agent/agent/scriptrunner"
	bosherr "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
)

type RunScriptAction struct {
	scriptProvider scriptrunner.JobScriptProvider
	specService    boshas.V1Service

	logTag string
	logger boshlog.Logger
}

func NewRunScript(
	scriptProvider scriptrunner.JobScriptProvider,
	specService boshas.V1Service,
	logger boshlog.Logger,
) RunScriptAction {
	return RunScriptAction{
		scriptProvider: scriptProvider,
		specService:    specService,

		logTag: "RunScript Action",
		logger: logger,
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

	scripts := a.findScripts(scriptName, currentSpec)

	a.logger.Info(a.logTag, "Will run script '%s' in '%d' jobs in parallel", scriptName, len(scripts))

	type scriptResult struct {
		Script scriptrunner.Script
		Error  error
	}

	resultChan := make(chan scriptResult)

	for _, script := range scripts {
		script := script
		go func() { resultChan <- scriptResult{script, script.Run()} }()
	}

	var failedScripts, passedScripts []string

	for i := 0; i < len(scripts); i++ {
		select {
		case r := <-resultChan:
			jobName := r.Script.Tag()
			if r.Error == nil {
				passedScripts = append(passedScripts, jobName)
				result[jobName] = "executed"
				a.logger.Info(a.logTag, "'%s' script has successfully executed", r.Script.Path())
			} else {
				failedScripts = append(failedScripts, jobName)
				result[jobName] = "failed"
				a.logger.Error(a.logTag, "'%s' script has failed with error: %s", r.Script.Path(), r.Error)
			}
		}
	}

	err = a.summarizeErrs(scriptName, passedScripts, failedScripts)

	return result, err
}

func (a RunScriptAction) findScripts(scriptName string, currentSpec boshas.V1ApplySpec) []scriptrunner.Script {
	var scripts []scriptrunner.Script

	for _, job := range currentSpec.Jobs() {
		script := a.scriptProvider.Get(job.BundleName(), scriptName)
		if script.Exists() {
			a.logger.Debug(a.logTag, "Found '%s' script in job '%s'", scriptName, job.BundleName())
			scripts = append(scripts, script)
		} else {
			a.logger.Debug(a.logTag, "Did not find '%s' script in job '%s'", scriptName, job.BundleName())
		}
	}

	return scripts
}

func (a RunScriptAction) summarizeErrs(scriptName string, passedScripts, failedScripts []string) error {
	if len(failedScripts) > 0 {
		errMsg := "Failed Jobs: " + strings.Join(failedScripts, ", ")

		if len(passedScripts) > 0 {
			errMsg += ". Successful Jobs: " + strings.Join(passedScripts, ", ")
		}

		total := len(failedScripts) + len(passedScripts)

		return bosherr.Errorf("%d of %d %s scripts failed. %s.", len(failedScripts), total, scriptName, errMsg)
	}

	return nil
}

func (a RunScriptAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a RunScriptAction) Cancel() error {
	return errors.New("not supported")
}
