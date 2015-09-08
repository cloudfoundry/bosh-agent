package scriptrunner

import (
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
)

type GenericScript struct {
	fs      system.FileSystem
	runner  system.CmdRunner
	path    string
	jobName string
}

func NewScript(
	fs system.FileSystem,
	runner system.CmdRunner,
	path string,
	jobName string,
) (script GenericScript) {
	script = GenericScript{
		fs:      fs,
		runner:  runner,
		path:    path,
		jobName: jobName,
	}
	return
}

func (script GenericScript) Path() string {
	return script.path
}

func (script GenericScript) JobName() string {
	return script.jobName
}

func (script GenericScript) Exists() bool {
	return script.fs.FileExists(script.Path())
}

func (script GenericScript) Run(errorChan chan RunScriptResult, doneChan chan RunScriptResult) {

	command := system.Command{
		Name: script.Path(),
		Env: map[string]string{
			"PATH": "/usr/sbin:/usr/bin:/sbin:/bin",
		},
	}

	_, _, _, err := script.runner.RunComplexCommand(command)

	if err == nil {
		doneChan <- RunScriptResult{script.JobName(), script.Path()}
	} else {
		errorChan <- RunScriptResult{script.JobName(), script.Path()}
	}
}
