package scriptrunner

import (
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
)

type GenericScript struct {
	fs     system.FileSystem
	runner system.CmdRunner
	path   string
}

func NewScript(
	fs system.FileSystem,
	runner system.CmdRunner,
	path string,
) (script GenericScript) {
	script = GenericScript{
		fs:     fs,
		runner: runner,
		path:   path,
	}
	return
}

func (script GenericScript) Path() string {
	return script.path
}

func (script GenericScript) Exists() bool {
	return script.fs.FileExists(script.Path())
}

func (script GenericScript) Run() (string, string, error) {

	command := system.Command{
		Name: script.Path(),
		Env: map[string]string{
			"PATH": "/usr/sbin:/usr/bin:/sbin:/bin",
		},
	}

	stdout, stderr, exitStatus, err := script.runner.RunComplexCommand(command)
	if err != nil {
		return stdout, stderr, errors.WrapError(err, "Running script")
	}

	if exitStatus != 0 {
		err = errors.WrapErrorf(err, "Script failed with status %d", exitStatus)
	}

	return stdout, stderr, err
}
