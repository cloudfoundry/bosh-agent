package action

import (
	"errors"
	"github.com/cloudfoundry/bosh-agent/agent/scriptrunner"
	bosherr "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
)

type RunScriptAction struct {
	scriptProvider scriptrunner.ScriptProvider
	logger         logger.Logger
}

func NewRunScript(
	scriptProvider scriptrunner.ScriptProvider,
	logger logger.Logger,
) RunScriptAction {
	return RunScriptAction{
		scriptProvider: scriptProvider,
		logger:         logger,
	}
}

func (a RunScriptAction) IsAsynchronous() bool {
	return true
}

func (a RunScriptAction) IsPersistent() bool {
	return false
}

func (a RunScriptAction) Run(scriptPaths []string, options map[string]interface{}) (string, error) {
	a.logger.Info("run-script-action", "Run Script command: %s", scriptPaths)

	script := a.scriptProvider.Get(scriptPaths[0])

	if !script.Exists() {
		return "missing", nil
	}

	_, _, err := script.Run()
	if err != nil {
		return "failed", bosherr.WrapError(err, "Running Script")
	}

	return "executed", nil
}

func (a RunScriptAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a RunScriptAction) Cancel() error {
	return errors.New("not supported")
}
