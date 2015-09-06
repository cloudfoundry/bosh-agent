package scriptrunner

import (
	"path/filepath"

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
	return NewScript(p.fs, p.cmdRunner, filepath.Join(p.dirProvider.JobBinDir(jobName), scriptName))
}
