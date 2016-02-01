package system

import (
	"bytes"
	"os/exec"
	"strings"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

const (
	execProcessLogTag = "Cmd Runner"
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
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	command := exec.Command(cmdName, args...)
	command.Stderr = stderr
	command.Stdout = stdout

	r.logger.Debug(execProcessLogTag, "Running command: %s %s", cmdName, strings.Join(args, " "))
	err := command.Run()
	r.logger.Debug(execProcessLogTag, "Stdout: %s", stdout)
	r.logger.Debug(execProcessLogTag, "Stderr: %s", stderr)
	if err != nil {
		return stdout.String(), stderr.String(), -1, err
	}

	return stdout.String(), stderr.String(), 0, nil
}

func (r execCmdRunner) RunCommandWithInput(input, cmdName string, args ...string) (string, string, int, error) {
	panic("Not implemented")
}

func (r execCmdRunner) CommandExists(cmdName string) bool {
	return true
}
