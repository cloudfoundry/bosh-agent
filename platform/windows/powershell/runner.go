package powershell

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry/bosh-utils/system"
)

const Executable = "powershell.exe"

type Runner struct {
	BaseCmdRunner system.CmdRunner
}

func (c *Runner) RunComplexCommand(cmd system.Command) (stdout, stderr string, exitStatus int, err error) {
	return c.BaseCmdRunner.RunComplexCommand(powershellCommand(cmd))
}

func (c *Runner) RunComplexCommandAsync(cmd system.Command) (system.Process, error) {
	return c.BaseCmdRunner.RunComplexCommandAsync(powershellCommand(cmd))
}

func (c *Runner) RunCommand(cmdName string, args ...string) (stdout, stderr string, exitStatus int, err error) {
	powershellArgs := append([]string{cmdName}, args...)
	stdout, stderr, exitStatus, err = c.BaseCmdRunner.RunCommand(Executable, powershellArgs...)
	if err != nil {
		commandRan := strings.Join(append([]string{Executable}, powershellArgs...), " ")

		if exitStatus == -1 {
			wrappedErr := fmt.Errorf("Failed to run command \"%s\": %s", commandRan, err.Error()) //nolint:staticcheck
			return stdout, stderr, exitStatus, wrappedErr
		}

		if exitStatus > 0 {
			wrappedErr := fmt.Errorf("Command \"%s\" exited with failure: %s", commandRan, stderr) //nolint:staticcheck
			return stdout, stderr, exitStatus, wrappedErr
		}
	}

	return
}

func (c *Runner) RunCommandQuietly(cmdName string, args ...string) (stdout, stderr string, exitStatus int, err error) {
	return c.BaseCmdRunner.RunCommandQuietly(Executable, append([]string{cmdName}, args...)...)
}

func (c *Runner) RunCommandWithInput(input, cmdName string, args ...string) (stdout, stderr string, exitStatus int, err error) {
	return c.BaseCmdRunner.RunCommandWithInput(input, Executable, append([]string{cmdName}, args...)...)
}

func (c *Runner) CommandExists(cmdName string) (exists bool) {
	_, _, exitStatus, _ := c.RunCommand("Get-Command", cmdName) //nolint:errcheck
	return exitStatus == 0
}

func powershellCommand(cmd system.Command) system.Command {
	newCmdArgs := append([]string{cmd.Name}, cmd.Args...)
	cmd.Name = Executable
	cmd.Args = newCmdArgs
	return cmd
}
