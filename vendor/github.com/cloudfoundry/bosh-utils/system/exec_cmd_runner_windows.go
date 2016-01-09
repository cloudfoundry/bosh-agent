package system

import boshlog "github.com/cloudfoundry/bosh-utils/logger"

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
	panic("Not implemented")
}

func (r execCmdRunner) RunCommandWithInput(input, cmdName string, args ...string) (string, string, int, error) {
	panic("Not implemented")
}

func (r execCmdRunner) CommandExists(cmdName string) bool {
	return true
}
