package script

import (
	"os"
	"path/filepath"

	"github.com/cloudfoundry/bosh-agent/v2/agent/script/cmd"
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

	env map[string]string
}

func NewScript(
	fs boshsys.FileSystem,
	runner boshsys.CmdRunner,
	tag string,
	path string,
	stdoutLogPath string,
	stderrLogPath string,
	env map[string]string,
) GenericScript {
	return GenericScript{
		fs:     fs,
		runner: runner,

		tag:  tag,
		path: path,

		stdoutLogPath: stdoutLogPath,
		stderrLogPath: stderrLogPath,

		env: env,
	}
}

func (s GenericScript) Tag() string  { return s.tag }
func (s GenericScript) Path() string { return s.path }
func (s GenericScript) Exists() bool { return s.fs.FileExists(s.path) }

func (s GenericScript) Run() error {
	err := s.ensureContainingDir(s.stdoutLogPath)
	if err != nil {
		return err
	}

	err = s.ensureContainingDir(s.stderrLogPath)
	if err != nil {
		return err
	}

	stdoutFile, err := s.fs.OpenFile(s.stdoutLogPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		return err
	}
	defer func() {
		_ = stdoutFile.Close()
	}()

	stderrFile, err := s.fs.OpenFile(s.stderrLogPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		return err
	}
	defer func() {
		_ = stderrFile.Close()
	}()

	command := cmd.BuildCommand(s.path)
	command.Stdout = stdoutFile
	command.Stderr = stderrFile

	for key, val := range s.env {
		command.Env[key] = val
	}

	_, _, _, err = s.runner.RunComplexCommand(command)

	return err
}

func (s GenericScript) ensureContainingDir(fullLogFilename string) error {
	dir, _ := filepath.Split(fullLogFilename)
	return s.fs.MkdirAll(dir, os.FileMode(0750))
}
