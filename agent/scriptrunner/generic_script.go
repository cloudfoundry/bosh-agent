package scriptrunner

import (
	"os"

	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
	"path/filepath"
)

const (
	fileOpenFlag int         = os.O_RDWR | os.O_CREATE | os.O_APPEND
	fileOpenPerm os.FileMode = os.FileMode(0640)
)

type GenericScript struct {
	tag           string
	fs            system.FileSystem
	runner        system.CmdRunner
	path          string
	stdoutLogPath string
	stderrLogPath string
}

func NewScript(
	tag string,
	fs system.FileSystem,
	runner system.CmdRunner,
	path string,
	stdoutLogPath string,
	stderrLogPath string,
) GenericScript {
	return GenericScript{
		tag:           tag,
		fs:            fs,
		runner:        runner,
		path:          path,
		stdoutLogPath: stdoutLogPath,
		stderrLogPath: stderrLogPath,
	}
}

func (script GenericScript) Path() string {
	return script.path
}

func (script GenericScript) Tag() string {
	return script.tag
}

func (script GenericScript) Exists() bool {
	return script.fs.FileExists(script.Path())
}

func (script GenericScript) Run(resultChannel chan RunScriptResult) {

	err := ensureContainingDir(script.fs, script.stdoutLogPath)
	if err != nil {
		resultChannel <- RunScriptResult{script.Tag(), script.Path(), err}
		return
	}

	err = ensureContainingDir(script.fs, script.stderrLogPath)
	if err != nil {
		resultChannel <- RunScriptResult{script.Tag(), script.Path(), err}
		return
	}

	stdoutFile, err := script.fs.OpenFile(script.stdoutLogPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		resultChannel <- RunScriptResult{script.Tag(), script.Path(), err}
		return
	}
	defer func() {
		_ = stdoutFile.Close()
	}()

	stderrFile, err := script.fs.OpenFile(script.stderrLogPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		resultChannel <- RunScriptResult{script.Tag(), script.Path(), err}
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
	resultChannel <- RunScriptResult{script.Tag(), script.Path(), runErr}
}

func ensureContainingDir(fs system.FileSystem, fullLogFilename string) error {
	dir, _ := filepath.Split(fullLogFilename)
	return fs.MkdirAll(dir, os.FileMode(0750))
}
