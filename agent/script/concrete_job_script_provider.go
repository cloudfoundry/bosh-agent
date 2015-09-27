package script

import (
	"fmt"
	"path/filepath"

	"github.com/cloudfoundry/bosh-agent/internal/github.com/pivotal-golang/clock"

	boshdrain "github.com/cloudfoundry/bosh-agent/agent/script/drain"
	boshlog "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
)

type ConcreteJobScriptProvider struct {
	cmdRunner   boshsys.CmdRunner
	fs          boshsys.FileSystem
	dirProvider boshdir.Provider
	timeService clock.Clock
	logger      boshlog.Logger
}

func NewConcreteJobScriptProvider(
	cmdRunner boshsys.CmdRunner,
	fs boshsys.FileSystem,
	dirProvider boshdir.Provider,
	timeService clock.Clock,
	logger boshlog.Logger,
) ConcreteJobScriptProvider {
	return ConcreteJobScriptProvider{
		cmdRunner:   cmdRunner,
		fs:          fs,
		dirProvider: dirProvider,
		timeService: timeService,
		logger:      logger,
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

func (p ConcreteJobScriptProvider) NewParallelScript(scriptName string, scripts []Script) Script {
	return NewParallelScript(scriptName, scripts, p.logger)
}
