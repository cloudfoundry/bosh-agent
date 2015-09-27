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

func (p ConcreteScriptProvider) NewScript(templateName string) Script {
	scriptPath := filepath.Join(p.dirProvider.JobsDir(), templateName, "bin", "drain")
	return NewConcreteScript(p.fs, p.cmdRunner, scriptPath, p.timeService)
}
