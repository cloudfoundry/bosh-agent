package installer

import (
	"errors"
	"fmt"
	"io"
	"path"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/system"
)

const InstallScriptName = "install.sh"

func New(system system.System) Installer {
	return &installer{system: system}
}

type Installer interface {
	Install(io.Reader) Error
}

type installer struct {
	system system.System
}

func (installer *installer) Install(reader io.Reader) Error {
	tmpDir, err := installer.system.TempDir("", "work-tmp")
	if err != nil {
		return installer.systemError(err)
	}

	result, err := installer.system.Untar(reader, tmpDir)
	if err != nil {
		return installer.systemError(err)
	}
	if result.ExitStatus != 0 {
		errorMessage := fmt.Sprintf("`%s` exited with %d", result.CommandRun, result.ExitStatus)
		return installer.userError(errorMessage)
	}

	if !installer.system.FileExists(path.Join(tmpDir, InstallScriptName)) {
		return installer.userError(fmt.Sprintf("No '%s' script found", InstallScriptName))
	}

	isExecutable, err := installer.system.FileIsExecutable(path.Join(tmpDir, InstallScriptName))
	if err != nil {
		return installer.systemError(err)
	}
	if !isExecutable {
		return installer.userError(fmt.Sprintf("'%s' is not executable", InstallScriptName))
	}

	result, err = installer.system.RunScript(fmt.Sprintf("./%s", InstallScriptName), tmpDir)
	if err != nil {
		return installer.systemError(err)
	}
	if result.ExitStatus != 0 {
		errorMessage := fmt.Sprintf("`%s` exited with %d", result.CommandRun, result.ExitStatus)
		return installer.userError(errorMessage)
	}

	return nil
}

func (installer *installer) systemError(err error) Error {
	return &installerError{err: err, isSystem: true}
}

func (installer *installer) userError(message string) Error {
	return &installerError{err: errors.New(message), isSystem: false}
}

type Error interface {
	error
	SystemError() bool
}

type installerError struct {
	err      error
	isSystem bool
}

func (installerError *installerError) Error() string {
	return installerError.err.Error()
}

func (installerError *installerError) SystemError() bool {
	return installerError.isSystem
}
