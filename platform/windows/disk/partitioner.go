package disk

import (
	"fmt"
	"strconv"
	"strings"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type Partitioner struct {
	Runner boshsys.CmdRunner
}

func (p *Partitioner) GetCountOnDisk(diskNumber string) (string, error) {
	getCountCommand := fmt.Sprintf(
		"Get-Disk -Number %s | Select -ExpandProperty NumberOfPartitions",
		diskNumber,
	)

	getCountCommandArgs := strings.Split(getCountCommand, " ")

	stdout, _, _, err := p.Runner.RunCommand(
		getCountCommandArgs[0],
		getCountCommandArgs[1:]...,
	)

	if err != nil {
		return "", fmt.Errorf("failed to get existing partition count for disk %s: %s", diskNumber, err)
	}

	return strings.TrimSpace(stdout), nil
}

func (p *Partitioner) GetFreeSpaceOnDisk(diskNumber string) (int, error) {
	getFreeSpaceCommand := fmt.Sprintf(
		"Get-Disk %s | Select -ExpandProperty LargestFreeExtent",
		diskNumber,
	)
	getFreeSpaceCommandArgs := strings.Split(getFreeSpaceCommand, " ")

	stdout, _, _, err := p.Runner.RunCommand(
		getFreeSpaceCommandArgs[0],
		getFreeSpaceCommandArgs[1:]...,
	)

	if err != nil {
		return 0, fmt.Errorf("failed to find free space on disk %s: %s", diskNumber, err)
	}

	stdoutTrimmed := strings.TrimSpace(stdout)
	freeSpace, err := strconv.Atoi(stdoutTrimmed)

	if err != nil {
		return 0, fmt.Errorf( //nolint:staticcheck
			"Failed to convert output of \"%s\" command in to number. Output was: \"%s\"",
			getFreeSpaceCommand,
			stdoutTrimmed,
		)
	}
	return freeSpace, nil
}

func (p *Partitioner) InitializeDisk(diskNumber string) error {
	_, _, _, err := p.Runner.RunCommand("Initialize-Disk", "-Number", diskNumber, "-PartitionStyle", "GPT")
	if err != nil {
		return fmt.Errorf("failed to initialize disk %s: %s", diskNumber, err.Error())
	}

	return nil
}

func (p *Partitioner) PartitionDisk(diskNumber string) (string, error) {
	stdout, _, _, err := p.Runner.RunCommand(
		"New-Partition",
		"-DiskNumber",
		diskNumber,
		"-UseMaximumSize",
		"|",
		"Select",
		"-ExpandProperty",
		"PartitionNumber",
	)
	if err != nil {
		return "", fmt.Errorf("failed to create partition on disk %s: %s", diskNumber, err)
	}

	return strings.TrimSpace(stdout), nil
}

func (p *Partitioner) AssignDriveLetter(diskNumber, partitionNumber string) (string, error) {
	_, _, _, err := p.Runner.RunCommand(
		"Add-PartitionAccessPath",
		"-DiskNumber",
		diskNumber,
		"-PartitionNumber",
		partitionNumber,
		"-AssignDriveLetter",
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to add partition access path to partition %s on disk %s: %s",
			partitionNumber,
			diskNumber,
			err,
		)
	}

	stdout, _, _, err := p.Runner.RunCommand(
		"Get-Partition",
		"-DiskNumber",
		diskNumber,
		"-PartitionNumber",
		partitionNumber,
		"|",
		"Select",
		"-ExpandProperty",
		"DriveLetter",
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to find drive letter for partition %s on disk %s: %s",
			partitionNumber,
			diskNumber,
			err,
		)
	}

	return strings.TrimSpace(stdout), nil
}
