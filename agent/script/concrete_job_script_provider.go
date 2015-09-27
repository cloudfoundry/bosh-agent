package script

import (
	"fmt"
	"path/filepath"

	boshsys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
)

type ConcreteJobScriptProvider struct {
	cmdRunner   boshsys.CmdRunner
	fs          boshsys.FileSystem
	dirProvider boshdir.Provider
}

func NewConcreteJobScriptProvider(
	cmdRunner boshsys.CmdRunner,
	fs boshsys.FileSystem,
	dirProvider boshdir.Provider,
) ConcreteJobScriptProvider {
	return ConcreteJobScriptProvider{
		cmdRunner:   cmdRunner,
		fs:          fs,
		dirProvider: dirProvider,
	}
}

func (p ConcreteJobScriptProvider) Get(jobName string, scriptName string) Script {
	stdoutLogFilename := fmt.Sprintf("%s.stdout.log", scriptName)
	stderrLogFilename := fmt.Sprintf("%s.stderr.log", scriptName)

	return NewScript(
		p.fs,
		p.cmdRunner,

		jobName,
		filepath.Join(p.dirProvider.JobBinDir(jobName), scriptName),

		filepath.Join(p.dirProvider.LogsDir(), jobName, stdoutLogFilename),
		filepath.Join(p.dirProvider.LogsDir(), jobName, stderrLogFilename),
	)
}
