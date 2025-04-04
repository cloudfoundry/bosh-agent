package disk

import (
	"fmt"
	"strings"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type Linker struct {
	Runner boshsys.CmdRunner
}

func (l *Linker) LinkTarget(location string) (target string, err error) {
	isLinkedCommand := fmt.Sprintf(
		"Get-Item %s -ErrorAction Ignore | Select -ExpandProperty Target -ErrorAction Ignore",
		location,
	)

	isLinkedCommandArgs := strings.Split(isLinkedCommand, " ")

	stdout, _, exitStatus, err := l.Runner.RunCommand(
		isLinkedCommandArgs[0],
		isLinkedCommandArgs[1:]...,
	)

	if err != nil && exitStatus == -1 {
		return "", fmt.Errorf("failed to check for existing symbolic link: %s", err)
	}

	return strings.TrimSpace(stdout), nil
}

func (l *Linker) Link(location, target string) error {
	createLinkCommand := fmt.Sprintf("cmd.exe /c mklink /d %s %s", location, target)
	createLinkCommandArgs := strings.Split(createLinkCommand, " ")

	_, _, _, err := l.Runner.RunCommand(
		createLinkCommandArgs[0],
		createLinkCommandArgs[1:]...,
	)

	if err != nil {
		return fmt.Errorf("failed to create symbolic link: %s", err)
	}

	return nil
}
