package drain

import (
	"strconv"
	"strings"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/pivotal-golang/clock"
)

type ConcreteScript struct {
	fs     boshsys.FileSystem
	runner boshsys.CmdRunner

	tag    string
	path   string
	params ScriptParams

	timeService clock.Clock
}

func NewConcreteScript(
	fs boshsys.FileSystem,
	runner boshsys.CmdRunner,
	tag string,
	path string,
	params ScriptParams,
	timeService clock.Clock,
) ConcreteScript {
	return ConcreteScript{
		fs:     fs,
		runner: runner,

		tag:    tag,
		path:   path,
		params: params,

		timeService: timeService,
	}
}

func (s ConcreteScript) Tag() string          { return s.tag }
func (s ConcreteScript) Path() string         { return s.path }
func (s ConcreteScript) Params() ScriptParams { return s.params }
func (s ConcreteScript) Exists() bool         { return s.fs.FileExists(s.path) }

func (s ConcreteScript) RunAsync() (boshsys.Process, boshsys.File, boshsys.File, error) {
	return nil, nil, nil, bosherr.Error("RunAsync not supported for drain scripts")
}

func (s ConcreteScript) Run() error {
	params := s.params

	for {
		value, err := s.runOnce(params)
		if err != nil {
			return err
		} else if value < 0 {
			s.timeService.Sleep(time.Duration(-value) * time.Second)
			params = params.ToStatusParams()
		} else {
			s.timeService.Sleep(time.Duration(value) * time.Second)
			return nil
		}
	}
}

func (s ConcreteScript) runOnce(params ScriptParams) (int, error) {
	jobChange := params.JobChange()
	hashChange := params.HashChange()
	updatedPkgs := params.UpdatedPackages()

	command := boshsys.Command{
		Name: s.path,
		Env: map[string]string{
			"PATH": "/usr/sbin:/usr/bin:/sbin:/bin",
		},
	}

	jobState, err := params.JobState()
	if err != nil {
		return 0, bosherr.WrapError(err, "Getting job state")
	}

	if jobState != "" {
		command.Env["BOSH_JOB_STATE"] = jobState
	}

	jobNextState, err := params.JobNextState()
	if err != nil {
		return 0, bosherr.WrapError(err, "Getting job next state")
	}

	if jobNextState != "" {
		command.Env["BOSH_JOB_NEXT_STATE"] = jobNextState
	}

	command.Args = append(command.Args, jobChange, hashChange)
	command.Args = append(command.Args, updatedPkgs...)

	stdout, _, _, err := s.runner.RunComplexCommand(command)
	if err != nil {
		return 0, bosherr.WrapError(err, "Running drain script")
	}

	value, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return 0, bosherr.WrapError(err, "Script did not return a signed integer")
	}

	return value, nil
}
