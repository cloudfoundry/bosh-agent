package scriptrunner

import (
	"os"
	"path/filepath"

	boshsys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
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

func (s GenericScript) Exists() bool { return s.fs.FileExists(s.path) }

func (s GenericScript) Run() ScriptResult {
	result := ScriptResult{
		Tag:        s.tag,
		ScriptPath: s.path,
	}

	err := s.ensureContainingDir(s.stdoutLogPath)
	if err != nil {
		result.Error = err
		return result
	}

	err = s.ensureContainingDir(s.stderrLogPath)
	if err != nil {
		result.Error = err
		return result
	}

	stdoutFile, err := s.fs.OpenFile(s.stdoutLogPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		result.Error = err
		return result
	}
	defer func() {
		_ = stdoutFile.Close()
	}()

	stderrFile, err := s.fs.OpenFile(s.stderrLogPath, fileOpenFlag, fileOpenPerm)
	if err != nil {
		result.Error = err
		return result
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

	_, _, _, err = s.runner.RunComplexCommand(command)
	result.Error = err

	return result
}

func (s GenericScript) ensureContainingDir(fullLogFilename string) error {
	dir, _ := filepath.Split(fullLogFilename)
	return s.fs.MkdirAll(dir, os.FileMode(0750))
}
