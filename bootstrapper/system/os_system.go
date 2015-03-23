package system

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type System interface {
	Untar(tarball io.Reader, targetDir string) (CommandResult, error)
	RunScript(scriptPath string, workingDir string) (CommandResult, error)
	TempDir(string, string) (string, error)
	FileExists(string) bool
}

type CommandResult struct {
	CommandRun string
	ExitStatus int
}

type osSystem struct {
}

func NewOsSystem() System {
	return &osSystem{}
}

func (system *osSystem) TempDir(dir string, prefix string) (string, error) {
	return ioutil.TempDir(dir, prefix)
}

func (system *osSystem) RunScript(scriptPath string, workingDir string) (CommandResult, error) {
	command := exec.Command(scriptPath)
	command.Dir = workingDir

	err := command.Start()
	if err != nil {
		return CommandResult{}, err
	}

	exitStatus := getExitStatus(command.Wait())
	return CommandResult{
		ExitStatus: exitStatus,
		CommandRun: strings.Join(command.Args, " "),
	}, nil
}

func (system *osSystem) Untar(tarball io.Reader, targetDir string) (CommandResult, error) {
	tarCommand := exec.Command("tar", "xvfz", "-")
	tarCommand.Dir = targetDir

	stdInPipe, err := tarCommand.StdinPipe()
	if err != nil {
		return CommandResult{}, err
	}

	err = tarCommand.Start()
	if err != nil {
		return CommandResult{}, err
	}

	_, err = io.Copy(stdInPipe, tarball)
	if err != nil {
		return CommandResult{}, err
	}

	err = stdInPipe.Close()
	if err != nil {
		return CommandResult{}, err
	}

	exitStatus := getExitStatus(tarCommand.Wait())
	return CommandResult{
		ExitStatus: exitStatus,
		CommandRun: strings.Join(tarCommand.Args, " "),
	}, nil
}

func (system *osSystem) FileExists(filepath string) bool {
	_, err := os.Stat(filepath)
	return err == nil
}

func getExitStatus(err error) int {
	if err == nil {
		return 0
	}

	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	panic(err)
}
