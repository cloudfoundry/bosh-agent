package scriptrunner

import (
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
	"github.com/cloudfoundry/bosh-agent/settings/directories"
)

type GenericScriptProvider struct {
	cmdRunner   system.CmdRunner
	fs          system.FileSystem
	dirProvider directories.Provider
}

func NewGenericScriptProvider(
	cmdRunner system.CmdRunner,
	fs system.FileSystem,
	dirProvider directories.Provider,
) (provider GenericScriptProvider) {
	provider.cmdRunner = cmdRunner
	provider.fs = fs
	provider.dirProvider = dirProvider
	return
}

func (p GenericScriptProvider) Get(scriptPath string) (script Script) {
	return NewScript(p.fs, p.cmdRunner, p.dirProvider, scriptPath)
}
