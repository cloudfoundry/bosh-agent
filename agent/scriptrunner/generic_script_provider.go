package scriptrunner

import (
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
)

type GenericScriptProvider struct {
	cmdRunner system.CmdRunner
	fs        system.FileSystem
}

func NewGenericScriptProvider(
	cmdRunner system.CmdRunner,
	fs system.FileSystem,
) (provider GenericScriptProvider) {
	provider.cmdRunner = cmdRunner
	provider.fs = fs
	return
}

func (p GenericScriptProvider) Get(scriptPath string) (script Script) {
	return NewScript(p.fs, p.cmdRunner, scriptPath)
}
