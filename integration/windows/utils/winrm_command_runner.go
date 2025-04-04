package utils

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/masterzen/winrm"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type WinRMCommandRunner struct {
	Client *winrm.Client
}

func (r *WinRMCommandRunner) RunComplexCommand(cmd boshsys.Command) (stdout, stderr string, exitStatus int, err error) {
	outBytes := &bytes.Buffer{}
	errBytes := &bytes.Buffer{}

	var exitCode int

	cmdName := cmd.Name
	cmdArgs := cmd.Args

	if cmdName == "powershell.exe" {
		cmdName = cmdArgs[0]
		cmdArgs = cmdArgs[1:]
	}

	powerShellCommand := winrm.Powershell(fmt.Sprintf("%s %s", cmdName, strings.Join(cmdArgs, " ")))

	if cmd.Stdin != nil {
		exitCode, err = r.Client.RunWithInput(powerShellCommand, outBytes, errBytes, cmd.Stdin) //nolint:staticcheck
	} else {
		exitCode, err = r.Client.Run(winrm.Powershell(powerShellCommand), outBytes, errBytes) //nolint:staticcheck
	}

	outString := outBytes.String()
	errString := errBytes.String()

	return outString, errString, exitCode, err
}

func (r *WinRMCommandRunner) RunComplexCommandAsync(cmd boshsys.Command) (boshsys.Process, error) {
	return nil, errors.New("Asynchronous commands not supported with this winRM implementation") //nolint:staticcheck
}

func (r *WinRMCommandRunner) RunCommand(cmdName string, args ...string) (stdout, stderr string, exitStatus int, err error) {
	return r.RunComplexCommand(boshsys.Command{Name: cmdName, Args: args})
}

func (r *WinRMCommandRunner) RunCommandQuietly(cmdName string, args ...string) (stdout, stderr string, exitStatus int, err error) {
	return r.RunComplexCommand(boshsys.Command{Name: cmdName, Args: args})
}

func (r *WinRMCommandRunner) RunCommandWithInput(input, cmdName string, args ...string) (stdout, stderr string, exitStatus int, err error) {
	return r.RunComplexCommand(boshsys.Command{Name: cmdName, Args: args, Stdin: strings.NewReader(input)})
}

func (r *WinRMCommandRunner) CommandExists(cmdName string) (exists bool) {
	return false
}
