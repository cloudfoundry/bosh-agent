package disk

import (
	"strings"

	"fmt"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type WindowsFormatter struct {
	Runner boshsys.CmdRunner
}

func (f *WindowsFormatter) Format(diskNumber, partitionNumber string) error {
	formatCommand := formatVolumeCommand(diskNumber, partitionNumber)
	formatCommandArgs := strings.Split(formatCommand, " ")

	_, stdErr, exitCode, rcErr := f.Runner.RunCommand(
		formatCommandArgs[0],
		formatCommandArgs[1:]...,
	)

	if rcErr != nil {
		return fmt.Errorf("Failed to run command \"%s\": %s", formatCommand, rcErr)
	}

	if exitCode != 0 {
		return fmt.Errorf(
			"Failed to format partition %s on disk %s: %s",
			partitionNumber, diskNumber, stdErr,
		)

	}

	return nil
}

func formatVolumeCommand(diskNumber, partitionNumber string) string {
	return fmt.Sprintf(
		"powershell.exe Get-Partition -DiskNumber %s -PartitionNumber %s | Format-Volume -FileSystem NTFS -Confirm:$false",
		diskNumber, partitionNumber,
	)
}
