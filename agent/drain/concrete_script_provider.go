package drain

import (
	"path/filepath"

	boshsys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/pivotal-golang/clock"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
)

type ConcreteScriptProvider struct {
	cmdRunner   boshsys.CmdRunner
	fs          boshsys.FileSystem
	dirProvider boshdirs.Provider
	timeService clock.Clock
}

func NewConcreteScriptProvider(
	cmdRunner boshsys.CmdRunner,
	fs boshsys.FileSystem,
	dirProvider boshdirs.Provider,
	timeService clock.Clock,
) ConcreteScriptProvider {
	return ConcreteScriptProvider{
		cmdRunner:   cmdRunner,
		fs:          fs,
		dirProvider: dirProvider,
		timeService: timeService,
	}
}

func (p ConcreteScriptProvider) NewScript(jobName string, params ScriptParams) Script {
	path := filepath.Join(p.dirProvider.JobsDir(), jobName, "bin", "drain")
	return NewConcreteScript(p.fs, p.cmdRunner, jobName, path, params, p.timeService)
}
