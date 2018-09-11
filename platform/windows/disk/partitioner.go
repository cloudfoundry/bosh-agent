package disk

import (
	"fmt"

	"strings"

	"strconv"

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

func (p *Partitioner) GetFreeSpaceOnDisk(diskNumber string) (int, error) {
	getFreeSpaceCommand := fmt.Sprintf(
		"powershell.exe Get-Disk %s | Select -ExpandProperty LargestFreeExtent",
		diskNumber,
	)
	getFreeSpaceCommandArgs := strings.Split(getFreeSpaceCommand, " ")

	stdout, stderr, exitStatus, err := p.Runner.RunCommand(
		getFreeSpaceCommandArgs[0],
		getFreeSpaceCommandArgs[1:]...,
	)

	if err != nil && exitStatus == -1 {
		return 0, fmt.Errorf("Failed to run command \"%s\": %s", getFreeSpaceCommand, err)
	}

	if exitStatus > 0 {
		return 0, fmt.Errorf("Command \"%s\" exited with failure: %s", getFreeSpaceCommand, stderr)
	}

	stdoutTrimmed := strings.TrimSpace(stdout)
	freeSpace, err := strconv.Atoi(stdoutTrimmed)

	if err != nil {
		return 0, fmt.Errorf(
			"Failed to convert output of \"%s\" command in to number. Output was: \"%s\"",
			getFreeSpaceCommand,
			stdoutTrimmed,
		)
	}
	return freeSpace, nil
}
