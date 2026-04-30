package disk

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

var diskNumberPattern = regexp.MustCompile(`^[0-9]+$`)

type Partitioner struct {
	Runner boshsys.CmdRunner
}

// ParseDiskNumberString validates a non-negative decimal disk or partition index for Windows PowerShell -Number parameters.
func ParseDiskNumberString(s string) (int, string, error) {
	s = strings.TrimSpace(s)
	if !diskNumberPattern.MatchString(s) {
		return 0, "", fmt.Errorf("disk number must be a non-negative decimal integer")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, "", fmt.Errorf("disk number must be a non-negative decimal integer")
	}
	return n, strconv.Itoa(n), nil
}

func canonicalDiskIndexString(s string) (int, string, error) {
	return ParseDiskNumberString(s)
}

func canonicalPartitionIndexString(s string) (int, string, error) {
	return ParseDiskNumberString(s)
}

func (p *Partitioner) ps(script string) (string, string, int, error) {
	return p.Runner.RunCommand("-NoProfile", "-NonInteractive", "-Command", script)
}

func (p *Partitioner) GetCountOnDisk(diskNumber string) (string, error) {
	n, _, err := canonicalDiskIndexString(diskNumber)
	if err != nil {
		return "", fmt.Errorf("GetCountOnDisk: invalid disk number %q: %w", diskNumber, err)
	}

	script := fmt.Sprintf(
		"Get-Disk -Number %d | Select-Object -ExpandProperty NumberOfPartitions",
		n,
	)

	stdout, _, _, err := p.ps(script)
	if err != nil {
		return "", fmt.Errorf("failed to get existing partition count for disk %s: %s", diskNumber, err)
	}

	return strings.TrimSpace(stdout), nil
}

func (p *Partitioner) GetFreeSpaceOnDisk(diskNumber string) (int, error) {
	n, _, err := canonicalDiskIndexString(diskNumber)
	if err != nil {
		return 0, fmt.Errorf("GetFreeSpaceOnDisk: invalid disk number %q: %w", diskNumber, err)
	}

	script := fmt.Sprintf(
		"Get-Disk -Number %d | Select-Object -ExpandProperty LargestFreeExtent",
		n,
	)

	stdout, _, _, err := p.ps(script)
	if err != nil {
		return 0, fmt.Errorf("failed to find free space on disk %s: %s", diskNumber, err)
	}

	stdoutTrimmed := strings.TrimSpace(stdout)
	freeSpace, err := strconv.Atoi(stdoutTrimmed)

	if err != nil {
		return 0, fmt.Errorf( //nolint:staticcheck
			"Failed to convert output of \"%s\" command in to number. Output was: \"%s\"",
			script,
			stdoutTrimmed,
		)
	}
	return freeSpace, nil
}

func (p *Partitioner) InitializeDisk(diskNumber string) error {
	n, _, err := canonicalDiskIndexString(diskNumber)
	if err != nil {
		return fmt.Errorf("InitializeDisk: invalid disk number %q: %w", diskNumber, err)
	}

	script := fmt.Sprintf("Initialize-Disk -Number %d -PartitionStyle GPT", n)
	_, _, _, err = p.ps(script)
	if err != nil {
		return fmt.Errorf("failed to initialize disk %s: %s", diskNumber, err.Error())
	}

	return nil
}

func (p *Partitioner) PartitionDisk(diskNumber string) (string, error) {
	n, _, err := canonicalDiskIndexString(diskNumber)
	if err != nil {
		return "", fmt.Errorf("PartitionDisk: invalid disk number %q: %w", diskNumber, err)
	}

	script := fmt.Sprintf(
		"New-Partition -DiskNumber %d -UseMaximumSize | Select-Object -ExpandProperty PartitionNumber",
		n,
	)

	stdout, _, _, err := p.ps(script)
	if err != nil {
		return "", fmt.Errorf("failed to create partition on disk %s: %s", diskNumber, err)
	}

	return strings.TrimSpace(stdout), nil
}

func (p *Partitioner) AssignDriveLetter(diskNumber, partitionNumber string) (string, error) {
	dn, _, err := canonicalDiskIndexString(diskNumber)
	if err != nil {
		return "", fmt.Errorf("AssignDriveLetter: invalid disk number %q: %w", diskNumber, err)
	}
	pn, _, err := canonicalPartitionIndexString(partitionNumber)
	if err != nil {
		return "", fmt.Errorf("AssignDriveLetter: invalid partition number %q: %w", partitionNumber, err)
	}

	addScript := fmt.Sprintf(
		"Add-PartitionAccessPath -DiskNumber %d -PartitionNumber %d -AssignDriveLetter",
		dn, pn,
	)
	_, _, _, err = p.ps(addScript)
	if err != nil {
		return "", fmt.Errorf(
			"failed to add partition access path to partition %s on disk %s: %s",
			partitionNumber,
			diskNumber,
			err,
		)
	}

	driveScript := fmt.Sprintf(
		"Get-Partition -DiskNumber %d -PartitionNumber %d | Select-Object -ExpandProperty DriveLetter",
		dn, pn,
	)

	stdout, _, _, err := p.ps(driveScript)
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
