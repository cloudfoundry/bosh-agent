package script

import (
	"fmt"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/clock"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshdrain "github.com/cloudfoundry/bosh-agent/v2/agent/script/drain"
	boshdir "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
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

func (p ConcreteJobScriptProvider) NewScript(jobName string, scriptName string, scriptEnv map[string]string) Script {
	path := path.Join(p.dirProvider.JobBinDir(jobName), scriptName+ScriptExt)

	stdoutLogFilename := fmt.Sprintf("%s.stdout.log", scriptName)
	stdoutLogPath := filepath.Join(p.dirProvider.LogsDir(), jobName, stdoutLogFilename)

	stderrLogFilename := fmt.Sprintf("%s.stderr.log", scriptName)
	stderrLogPath := filepath.Join(p.dirProvider.LogsDir(), jobName, stderrLogFilename)

	return NewScript(p.fs, p.cmdRunner, jobName, path, stdoutLogPath, stderrLogPath, scriptEnv)
}

func (p ConcreteJobScriptProvider) NewDrainScript(jobName string, params boshdrain.ScriptParams) CancellableScript {
	path := path.Join(p.dirProvider.JobsDir(), jobName, "bin", "drain"+ScriptExt)

	return boshdrain.NewConcreteScript(p.fs, p.cmdRunner, jobName, path, params, p.timeService, p.logger)
}

func (p ConcreteJobScriptProvider) NewParallelScript(scriptName string, scripts []Script) CancellableScript {
	return NewParallelScript(scriptName, scripts, p.logger)
}
