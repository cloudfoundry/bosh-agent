package disk

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry/bosh-utils/system"
)

const ProtectCmdlet = "Protect-Path"

type Protector struct {
	Runner system.CmdRunner
}

func (p *Protector) CommandExists() bool {
	return p.Runner.CommandExists(ProtectCmdlet)
}

func (p *Protector) ProtectPath(path string) error {
	_, _, _, err := p.Runner.RunCommand(ProtectCmdlet, fmt.Sprintf(`'%s'`, strings.TrimRight(path, "\\")))
	if err != nil {
		return fmt.Errorf("failed to protect '%s': %s", path, err)
	}

	return nil
}
