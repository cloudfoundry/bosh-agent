package script

import (
	"os"
	"path/filepath"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const (
	fileOpenFlag int         = os.O_RDWR | os.O_CREATE | os.O_APPEND
	fileOpenPerm os.FileMode = os.FileMode(0640)
)

type GenericScript struct {
	fs     boshsys.FileSystem
	runner boshsys.CmdRunner

	tag  string
	path string

	stdoutLogPath string
	stderrLogPath string
}

func NewScript(
	fs boshsys.FileSystem,
	runner boshsys.CmdRunner,
	tag string,
	path string,
	stdoutLogPath string,
	stderrLogPath string,
) GenericScript {
	return GenericScript{
		fs:     fs,
		runner: runner,

		tag:  tag,
		path: path,

		stdoutLogPath: stdoutLogPath,
		stderrLogPath: stderrLogPath,
	}
}

func (s GenericScript) Tag() string  { return s.tag }
func (s GenericScript) Path() string { return s.path }
func (s GenericScript) Exists() bool { return s.fs.FileExists(s.path) }

func (s GenericScript) Run() error {
	command, stdout, stderr, err := s.prepareCommand()

	if err != nil {
		return err
	}

	defer func() {
		if stdout != nil {
			_ = stdout.Close()
		}
	}()

	defer func() {
		if stderr != nil {
			_ = stderr.Close()
		}
	}()

	_, _, _, err = s.runner.RunComplexCommand(command)

	return err
}

func (s GenericScript) RunAsync() (boshsys.Process, boshsys.File, boshsys.File, error) {
	command, stdout, stderr, err := s.prepareCommand()

	if err != nil {
		return nil, nil, nil, err
	}

	process, err := s.runner.RunComplexCommandAsync(command)

	return process, stdout, stderr, err
}

func (s GenericScript) prepareCommand() (boshsys.Command, boshsys.File, boshsys.File, error) {
	err := s.ensureContainingDir(s.stdoutLogPath)
	if err != nil {
		return boshsys.Command{}, nil, nil, err
	}

	err = s.ensureContainingDir(s.stderrLogPath)
	if err != nil {
		return boshsys.Command{}, nil, nil, err
	}

	stdoutFile, err := s.fs.OpenFile(s.stdoutLogPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		return boshsys.Command{}, nil, nil, err
	}

	stderrFile, err := s.fs.OpenFile(s.stderrLogPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		return boshsys.Command{}, nil, nil, err
	}

	command := boshsys.Command{
		Name: s.path,
		Env: map[string]string{
			"PATH": "/usr/sbin:/usr/bin:/sbin:/bin",
		},
		Stdout: stdoutFile,
		Stderr: stderrFile,
	}

	return command, stdoutFile, stderrFile, nil
}

func (s GenericScript) ensureContainingDir(fullLogFilename string) error {
	dir, _ := filepath.Split(fullLogFilename)
	return s.fs.MkdirAll(dir, os.FileMode(0750))
}
