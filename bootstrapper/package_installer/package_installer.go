package package_installer

import (
	"errors"
	"fmt"
	"io"
	"path"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/system"
)

const InstallScriptName = "install.sh"

func New(system system.System) PackageInstaller {
	return &packageInstaller{system: system}
}

type PackageInstaller interface {
	Install(io.Reader) PackageInstallerError
}

type packageInstaller struct {
	system system.System
}

func (packageInstaller *packageInstaller) Install(reader io.Reader) PackageInstallerError {
	tmpDir, err := packageInstaller.system.TempDir("", "work-tmp")
	if err != nil {
		return packageInstaller.systemError(err)
	}

	result, err := packageInstaller.system.Untar(reader, tmpDir)
	if err != nil {
		return packageInstaller.systemError(err)
	}
	if result.ExitStatus != 0 {
		errorMessage := fmt.Sprintf("`%s` exited with %d", result.CommandRun, result.ExitStatus)
		return packageInstaller.userError(errorMessage)
	}

	if !packageInstaller.system.FileExists(path.Join(tmpDir, InstallScriptName)) {
		return packageInstaller.userError(fmt.Sprintf("No '%s' script found", InstallScriptName))
	}

	isExecutable, err := packageInstaller.system.FileIsExecutable(path.Join(tmpDir, InstallScriptName))
	if err != nil {
		return packageInstaller.systemError(err)
	}
	if !isExecutable {
		return packageInstaller.userError(fmt.Sprintf("'%s' is not executable", InstallScriptName))
	}

	result, err = packageInstaller.system.RunScript(fmt.Sprintf("./%s", InstallScriptName), tmpDir)
	if err != nil {
		return packageInstaller.systemError(err)
	}
	if result.ExitStatus != 0 {
		errorMessage := fmt.Sprintf("`%s` exited with %d", result.CommandRun, result.ExitStatus)
		return packageInstaller.userError(errorMessage)
	}

	return nil
}

func (packageInstaller *packageInstaller) systemError(err error) PackageInstallerError {
	return &packageInstallerError{err: err, isSystem: true}
}

func (packageInstaller *packageInstaller) userError(message string) PackageInstallerError {
	return &packageInstallerError{err: errors.New(message), isSystem: false}
}

type PackageInstallerError interface {
	error
	SystemError() bool
}

type packageInstallerError struct {
	err      error
	isSystem bool
}

func (packageInstallerError *packageInstallerError) Error() string {
	return packageInstallerError.err.Error()
}

func (packageInstallerError *packageInstallerError) SystemError() bool {
	return packageInstallerError.isSystem
}
