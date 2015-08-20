package scriptrunner

import (
	"strconv"
	"strings"

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

func (script GenericScript) Exists() bool {
	return script.fs.FileExists(script.path)
}

func (script GenericScript) Run() (int, error) {

	command := system.Command{
		Name: script.path,
		Env: map[string]string{
			"PATH": "/usr/sbin:/usr/bin:/sbin:/bin",
		},
	}

	stdout, _, _, err := script.runner.RunComplexCommand(command)
	if err != nil {
		return 0, errors.WrapError(err, "Running script")
	}

	value, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return 0, errors.WrapError(err, "Script did not return a signed integer")
	}

	return value, nil
}
