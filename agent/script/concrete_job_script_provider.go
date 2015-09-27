package script

import (
	"fmt"
	"path/filepath"

	"github.com/cloudfoundry/bosh-agent/internal/github.com/pivotal-golang/clock"

	boshdrain "github.com/cloudfoundry/bosh-agent/agent/script/drain"
	boshsys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
)

type ConcreteJobScriptProvider struct {
	cmdRunner   boshsys.CmdRunner
	fs          boshsys.FileSystem
	dirProvider boshdir.Provider
	timeService clock.Clock
}

func NewConcreteJobScriptProvider(
	cmdRunner boshsys.CmdRunner,
	fs boshsys.FileSystem,
	dirProvider boshdir.Provider,
	timeService clock.Clock,
) ConcreteJobScriptProvider {
	return ConcreteJobScriptProvider{
		cmdRunner:   cmdRunner,
		fs:          fs,
		dirProvider: dirProvider,
		timeService: timeService,
	}
}

func (p ConcreteJobScriptProvider) NewScript(jobName string, scriptName string) Script {
	path := filepath.Join(p.dirProvider.JobBinDir(jobName), scriptName)

	stdoutLogFilename := fmt.Sprintf("%s.stdout.log", scriptName)
	stdoutLogPath := filepath.Join(p.dirProvider.LogsDir(), jobName, stdoutLogFilename)

	stderrLogFilename := fmt.Sprintf("%s.stderr.log", scriptName)
	stderrLogPath := filepath.Join(p.dirProvider.LogsDir(), jobName, stderrLogFilename)

	return NewScript(p.fs, p.cmdRunner, jobName, path, stdoutLogPath, stderrLogPath)
}

func (p ConcreteJobScriptProvider) NewDrainScript(jobName string, params boshdrain.ScriptParams) Script {
	path := filepath.Join(p.dirProvider.JobsDir(), jobName, "bin", "drain")

	return boshdrain.NewConcreteScript(p.fs, p.cmdRunner, jobName, path, params, p.timeService)
}
