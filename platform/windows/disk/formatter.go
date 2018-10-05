package disk

import (
	"strings"

	"fmt"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type Formatter struct {
	Runner boshsys.CmdRunner
}

func (f *Formatter) Format(diskNumber, partitionNumber string) error {
	formatCommand := formatVolumeCommand(diskNumber, partitionNumber)
	formatCommandArgs := strings.Split(formatCommand, " ")

	_, _, _, err := f.Runner.RunCommand(
		formatCommandArgs[0],
		formatCommandArgs[1:]...,
	)

	if err != nil {
		return fmt.Errorf("failed to format volume: %s", err)
	}

	return nil
}

func formatVolumeCommand(diskNumber, partitionNumber string) string {
	return fmt.Sprintf(
		"Get-Partition -DiskNumber %s -PartitionNumber %s | Format-Volume -FileSystem NTFS -Confirm:$false",
		diskNumber, partitionNumber,
	)
}
