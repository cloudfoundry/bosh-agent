package system

import (
	"bytes"
	"fmt"
	"os"
	// "os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

const (
	execProcessLogTag      = "Cmd Runner"
	execErrorMsgFmt        = "Running command: '%s', stdout: '%s', stderr: '%s'"
	execShortErrorMaxLines = 100
)

type ExecError struct {
	Command string
	StdOut  string
	StdErr  string
}

func NewExecError(cmd, stdout, stderr string) ExecError {
	return ExecError{
		Command: cmd,
		StdOut:  stdout,
		StdErr:  stderr,
	}
}

func (e ExecError) Error() string {
	return fmt.Sprintf(execErrorMsgFmt, e.Command, e.StdOut, e.StdErr)
}

type windowsProcess struct {
	cmd          *exec.Cmd
	stdoutWriter *bytes.Buffer
	stderrWriter *bytes.Buffer
	pid          int
	logger       boshlog.Logger
	waitCh       chan Result
}

func newWindowsProcess(cmd *exec.Cmd, logger boshlog.Logger) *windowsProcess {
	return &windowsProcess{
		cmd:          cmd,
		stdoutWriter: new(bytes.Buffer),
		stderrWriter: new(bytes.Buffer),
		logger:       logger,
	}
}

func (p *windowsProcess) Start() error {
	// TODO (CEV): Is this required?
	if p.cmd.Stdout == nil {
		p.cmd.Stdout = p.stdoutWriter
	}
	if p.cmd.Stderr == nil {
		p.cmd.Stderr = p.stderrWriter
	}
	cmdString := strings.Join(p.cmd.Args, " ")
	p.logger.Debug(execProcessLogTag, "Running command: %s", cmdString)

	err := p.cmd.Start()
	if err != nil {
		return bosherr.WrapErrorf(err, "Starting command %s", cmdString)
	}

	p.pid = p.cmd.Process.Pid
	return nil
}

func (p *windowsProcess) Wait() <-chan Result {
	p.waitCh = make(chan Result, 1)
	go func() {
		p.waitCh <- p.wait()
	}()
	return p.waitCh
}

func (p *windowsProcess) wait() Result {
	err := p.cmd.Wait()

	stdout := p.stdoutWriter.String()
	p.logger.Debug(execProcessLogTag, "Stdout: %s", stdout)

	stderr := p.stderrWriter.String()
	p.logger.Debug(execProcessLogTag, "Stderr: %s", stderr)

	sys := p.cmd.ProcessState.Sys()
	waitStatus, ok := p.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		// WARN
		panic(fmt.Sprintf("%#v", sys))
	}
	var exitStatus int
	switch {
	case waitStatus.Exited():
		exitStatus = waitStatus.ExitStatus()
	case waitStatus.Signaled():
		exitStatus = 128 + int(waitStatus.Signal())
	default:
		exitStatus = -1
	}
	p.logger.Debug(execProcessLogTag, "Successful: %t (%d)", err == nil, exitStatus)

	if err != nil {
		cmdString := strings.Join(p.cmd.Args, " ")
		err = bosherr.WrapComplexError(err, NewExecError(cmdString, stdout, stderr))
	}

	return Result{
		Stdout:     stdout,
		Stderr:     stderr,
		ExitStatus: exitStatus,
		Error:      err,
	}
}

func (p *windowsProcess) TerminateNicely(killGracePeriod time.Duration) error {
	panic("windowsProcess: TerminateNicely NOT IMPLEMENTED")
	return nil
}

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
	process := newWindowsProcess(r.buildComplexCommand(cmd), r.logger)

	err := process.Start()
	if err != nil {
		return nil, err
	}

	return process, nil
}

func (e *execCmdRunner) buildComplexCommand(cmd Command) *exec.Cmd {
	execCmd := exec.Command(cmd.Name, cmd.Args...)
	execCmd.Stdin = cmd.Stdin
	execCmd.Stdout = cmd.Stdout
	execCmd.Stderr = cmd.Stderr
	execCmd.Dir = cmd.WorkingDir

	var env []string
	if !cmd.UseIsolatedEnv {
		env = os.Environ()
	}
	for k, v := range cmd.Env {
		env = append(env, k+"="+v)
	}
	execCmd.Env = env

	return execCmd
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
