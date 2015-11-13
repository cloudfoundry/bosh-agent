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
	command, err := s.prepareCommand()

	if err != nil {
		return err
	}

	_, _, _, err = s.runner.RunComplexCommand(command)

	return err
}

func (s GenericScript) RunAsync() (boshsys.Process, error) {
	command, err := s.prepareCommand()

	if err != nil {
		return nil, err
	}

	return s.runner.RunComplexCommandAsync(command)
}

func (s GenericScript) prepareCommand() (boshsys.Command, error) {
	err := s.ensureContainingDir(s.stdoutLogPath)
	if err != nil {
		return boshsys.Command{}, err
	}

	err = s.ensureContainingDir(s.stderrLogPath)
	if err != nil {
		return boshsys.Command{}, err
	}

	stdoutFile, err := s.fs.OpenFile(s.stdoutLogPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		return boshsys.Command{}, err
	}
	defer func() {
		_ = stdoutFile.Close()
	}()

	stderrFile, err := s.fs.OpenFile(s.stderrLogPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		return boshsys.Command{}, err
	}
	defer func() {
		_ = stderrFile.Close()
	}()

	command := boshsys.Command{
		Name: s.path,
		Env: map[string]string{
			"PATH": "/usr/sbin:/usr/bin:/sbin:/bin",
		},
		Stdout: stdoutFile,
		Stderr: stderrFile,
	}

	return command, nil
}

func (s GenericScript) ensureContainingDir(fullLogFilename string) error {
	dir, _ := filepath.Split(fullLogFilename)
	return s.fs.MkdirAll(dir, os.FileMode(0750))
}
