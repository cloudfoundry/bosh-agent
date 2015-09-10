package scriptrunner

import (
	"fmt"
	"os"

	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
)

const (
	fileOpenFlag int         = os.O_RDWR | os.O_CREATE | os.O_APPEND
	fileOpenPerm os.FileMode = os.FileMode(0640)
)

type GenericScript struct {
	fs      system.FileSystem
	runner  system.CmdRunner
	path    string
	logPath string
	jobName string
}

func NewScript(
	fs system.FileSystem,
	runner system.CmdRunner,
	path string,
	logPath string,
	jobName string,
) (script GenericScript) {
	script = GenericScript{
		fs:      fs,
		runner:  runner,
		path:    path,
		logPath: logPath,
		jobName: jobName,
	}
	return
}

func (script GenericScript) Path() string {
	return script.path
}

func (script GenericScript) LogPath() string {
	return script.logPath
}

func (script GenericScript) JobName() string {
	return script.jobName
}

func (script GenericScript) Exists() bool {
	return script.fs.FileExists(script.Path())
}

func (script GenericScript) Run(errorChan chan RunScriptResult, doneChan chan RunScriptResult) {

	err := script.fs.MkdirAll(script.LogPath(), os.FileMode(0750))
	if err != nil {
		errorChan <- RunScriptResult{script.JobName(), script.Path(), err}
		return
	}

	stdoutPath := fmt.Sprintf("%s.stdout.log", script.LogPath())
	stdoutFile, err := script.fs.OpenFile(stdoutPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		errorChan <- RunScriptResult{script.JobName(), script.Path(), err}
		return
	}
	defer func() {
		_ = stdoutFile.Close()
	}()

	stderrPath := fmt.Sprintf("%s.stderr.log", script.LogPath())
	stderrFile, err := script.fs.OpenFile(stderrPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		errorChan <- RunScriptResult{script.JobName(), script.Path(), err}
		return
	}
	defer func() {
		_ = stderrFile.Close()
	}()

	command := system.Command{
		Name: script.Path(),
		Env: map[string]string{
			"PATH": "/usr/sbin:/usr/bin:/sbin:/bin",
		},
	}
	command.Stdout = stdoutFile
	command.Stderr = stderrFile

	_, _, _, runErr := script.runner.RunComplexCommand(command)

	if runErr == nil {
		doneChan <- RunScriptResult{script.JobName(), script.Path(), nil}
	} else {
		errorChan <- RunScriptResult{script.JobName(), script.Path(), runErr}
	}
}
