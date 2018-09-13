package disk

import "github.com/cloudfoundry/bosh-utils/system"

const ProtectCmdlet = "Protect-Path"

type Protector struct {
	Runner system.CmdRunner
}

func (p *Protector) CommandExists() bool {
	return p.Runner.CommandExists(ProtectCmdlet)
}
