package system

import (
	"bytes"
	"os/exec"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type execCmdRunner struct {
	logger boshlog.Logger
}

func NewExecCmdRunner(logger boshlog.Logger) CmdRunner {
	return execCmdRunner{logger}
}

func (r execCmdRunner) RunComplexCommand(cmd Command) (string, string, int, error) {
	panic("Not implemented")
}

func (r execCmdRunner) RunComplexCommandAsync(cmd Command) (Process, error) {
	panic("Not implemented")
}

func (r execCmdRunner) RunCommand(cmdName string, args ...string) (string, string, int, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	command := exec.Command(cmdName, args...)
	command.Stderr = stdErr
	command.Stdout = stdOut

	err := command.Run()
	if err != nil {
		return "", "", -1, err
	}

	if err != nil {
		return stdOut.String(), stdErr.String(), -1, err
	}

	return stdOut.String(), stdErr.String(), 0, nil
}

func (r execCmdRunner) RunCommandWithInput(input, cmdName string, args ...string) (string, string, int, error) {
	panic("Not implemented")
}

func (r execCmdRunner) CommandExists(cmdName string) bool {
	return true
}
