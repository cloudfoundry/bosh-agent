package action

import (
	"errors"

	boshas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	boshscript "github.com/cloudfoundry/bosh-agent/v2/agent/script"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type RunScriptOptions struct {
	Env map[string]string `json:"env"`
}

type RunScriptAction struct {
	scriptProvider boshscript.JobScriptProvider
	specService    boshas.V1Service

	logTag string
	logger boshlog.Logger
}

func NewRunScript(
	scriptProvider boshscript.JobScriptProvider,
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

func (a RunScriptAction) IsAsynchronous(_ ProtocolVersion) bool {
	return true
}

func (a RunScriptAction) IsPersistent() bool {
	return false
}

func (a RunScriptAction) IsLoggable() bool {
	return true
}

func (a RunScriptAction) Run(scriptName string, options RunScriptOptions) (map[string]string, error) {
	// May be used in future to return more information
	emptyResults := map[string]string{}

	currentSpec, err := a.specService.Get()
	if err != nil {
		return emptyResults, bosherr.WrapError(err, "Getting current spec")
	}

	scripts := make([]boshscript.Script, 0, len(currentSpec.Jobs()))
	for _, job := range currentSpec.Jobs() {
		script := a.scriptProvider.NewScript(job.BundleName(), scriptName, options.Env)
		scripts = append(scripts, script)
	}

	parallelScript := a.scriptProvider.NewParallelScript(scriptName, scripts)

	return emptyResults, parallelScript.Run()
}

func (a RunScriptAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a RunScriptAction) Cancel() error {
	return errors.New("not supported")
}
