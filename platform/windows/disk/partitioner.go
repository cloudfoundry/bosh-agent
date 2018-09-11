package disk

import (
	"fmt"

	"strings"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type Partitioner struct {
	Runner boshsys.CmdRunner
}

func (p *Partitioner) GetCountOnDisk(diskNumber string) (string, error) {
	getCountCommand := fmt.Sprintf(
		"powershell.exe Get-Disk -Number %s | Select -ExpandProperty NumberOfPartitions",
		diskNumber,
	)

	getCountCommandArgs := strings.Split(getCountCommand, " ")

	stdout, stderr, exitStatus, err := p.Runner.RunCommand(
		getCountCommandArgs[0],
		getCountCommandArgs[1:]...,
	)

	if err != nil && exitStatus == -1 {
		return "", fmt.Errorf("Failed to run command \"%s\": %s", getCountCommand, err)
	}

	if exitStatus > 0 {
		return "", fmt.Errorf("Command \"%s\" exited with failure: %s", getCountCommand, stderr)
	}

	return strings.TrimSpace(stdout), nil
}
