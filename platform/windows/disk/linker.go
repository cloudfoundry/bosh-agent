package disk

import (
	"strings"

	"fmt"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type Linker struct {
	Runner boshsys.CmdRunner
}

func (l *Linker) IsLinked(location string) (target string, err error) {
	isLinkedCommand := fmt.Sprintf(
		"powershell.exe Get-Item %s -ErrorAction Ignore | Select -ExpandProperty Target -ErrorAction Ignore",
		location,
	)

	isLinkedCommandArgs := strings.Split(isLinkedCommand, " ")

	stdout, _, exitStatus, err := l.Runner.RunCommand(
		isLinkedCommandArgs[0],
		isLinkedCommandArgs[1:]...,
	)

	if err != nil && exitStatus == -1 {
		return "", fmt.Errorf("Failed to run command \"%s\": %s", isLinkedCommand, err)
	}

	return strings.TrimSpace(stdout), nil
}
