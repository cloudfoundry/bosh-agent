package system

import (
	"bytes"
	"os/exec"
	"syscall"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type concretePSRunner struct {
	fs     FileSystem
	logTag string
	logger boshlog.Logger
}

func NewConcretePSRunner(fs FileSystem, logger boshlog.Logger) PSRunner {
	return concretePSRunner{
		fs:     fs,
		logTag: "concretePSRunner",
		logger: logger,
	}
}

func (r concretePSRunner) RunCommand(cmd PSCommand) (string, string, error) {
	file, err := r.fs.TempFile(r.logTag)
	if err != nil {
		return "", "", bosherr.WrapError(err, "Creating tempfile")
	}

	_, err = file.Write([]byte(cmd.Script))
	if err != nil {
		return "", "", bosherr.WrapError(err, "Writing to tempfile")
	}

	err = file.Close()
	if err != nil {
		return "", "", bosherr.WrapError(err, "Closing tempfile")
	}

	err = r.fs.Rename(file.Name(), file.Name()+".ps1")
	if err != nil {
		return "", "", bosherr.WrapError(err, "Renaming tempfile")
	}

	execCmd := exec.Command("powershell", "-noprofile", "-noninteractive", file.Name()+".ps1")

	execCmd.Stdin = bytes.NewBufferString(cmd.Script)

	stdout := bytes.NewBufferString("")
	execCmd.Stdout = stdout

	stderr := bytes.NewBufferString("")
	execCmd.Stderr = stderr

	r.logger.DebugWithDetails(r.logTag, "Running script", cmd.Script)
	err = execCmd.Run()

	stdoutStr := string(stdout.Bytes())
	r.logger.DebugWithDetails(r.logTag, "Stdout", stdoutStr)

	stderrStr := string(stderr.Bytes())
	r.logger.DebugWithDetails(r.logTag, "Stderr", stderrStr)

	waitStatus := execCmd.ProcessState.Sys().(syscall.WaitStatus)

	r.logger.Debug(r.logTag, "Successful: %t (%d)", err == nil, waitStatus.ExitStatus())

	return stdoutStr, stderrStr, err
}
