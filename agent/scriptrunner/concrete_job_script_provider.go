package scriptrunner

import (
	"path/filepath"

	"fmt"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
	"github.com/cloudfoundry/bosh-agent/settings/directories"
)

type ConcreteJobScriptProvider struct {
	cmdRunner   system.CmdRunner
	fs          system.FileSystem
	dirProvider directories.Provider
}

func NewJobScriptProvider(
	cmdRunner system.CmdRunner,
	fs system.FileSystem,
	dirProvider directories.Provider,
) (provider ConcreteJobScriptProvider) {
	provider.cmdRunner = cmdRunner
	provider.fs = fs
	provider.dirProvider = dirProvider
	return
}

func (p ConcreteJobScriptProvider) Get(jobName string, scriptName string) (script Script) {
	stdoutLogFilename := fmt.Sprintf("%s.stdout.log", scriptName)
	stderrLogFilename := fmt.Sprintf("%s.stderr.log", scriptName)
	return NewScript(
		jobName,
		p.fs,
		p.cmdRunner,
		filepath.Join(p.dirProvider.JobBinDir(jobName), scriptName),
		filepath.Join(p.dirProvider.LogsDir(), jobName, stdoutLogFilename),
		filepath.Join(p.dirProvider.LogsDir(), jobName, stderrLogFilename),
	)
}
