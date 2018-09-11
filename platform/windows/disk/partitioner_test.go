package disk_test

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry/bosh-agent/platform/windows/disk"
	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Partitioner", func() {
	var (
		cmdRunner   *fakes.FakeCmdRunner
		partitioner *disk.Partitioner
		diskNumber  string
	)

	BeforeEach(func() {
		cmdRunner = fakes.NewFakeCmdRunner()
		partitioner = &disk.Partitioner{
			Runner: cmdRunner,
		}
		diskNumber = "1"
	})

	Describe("GetCountOnDisk", func() {
		It("returns number of partitions found on disk", func() {
			expectedPartitionCount := "2"

			cmdRunner.AddCmdResult(
				fmt.Sprintf("powershell.exe Get-Disk -Number %s | Select -ExpandProperty NumberOfPartitions", diskNumber),
				fakes.FakeCmdResult{
					Stdout: fmt.Sprintf(`%s
`, expectedPartitionCount),
				},
			)

			partitionCount, err := partitioner.GetCountOnDisk(diskNumber)
			Expect(err).NotTo(HaveOccurred())
			Expect(partitionCount).To(Equal(expectedPartitionCount))
		})

		It("when the command fails to run, returns a wrapped error", func() {
			cmdRunnerError := errors.New("It went wrong")
			countCommand := partitionCountCommand(diskNumber)

			cmdRunner.AddCmdResult(
				countCommand,
				fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
			)

			_, err := partitioner.GetCountOnDisk(diskNumber)
			Expect(err).To(MatchError(fmt.Sprintf(
				"Failed to run command \"%s\": It went wrong",
				countCommand,
			)))
		})

		It("when command runs but returns non-zero exit code, returns the command standard error", func() {
			cmdStandardError := `Get-Disk : No MSFT_Disk objects found with property 'Number' equal to '0'.
Verify the value of the property and retry.
At line:1 char:1
+ Get-Disk 0
+ ~~~~~~~~~~
    + CategoryInfo          : ObjectNotFound: (0:UInt32) [Get-Disk], CimJobExc
   eption
    + FullyQualifiedErrorId : CmdletizationQuery_NotFound_Number,Get-Disk
`
			countCommand := partitionCountCommand(diskNumber)

			cmdRunner.AddCmdResult(
				countCommand,
				fakes.FakeCmdResult{ExitStatus: 197, Stderr: cmdStandardError, Error: commandExitError},
			)

			_, err := partitioner.GetCountOnDisk(diskNumber)
			Expect(err).To(MatchError(fmt.Sprintf(
				"Command \"%s\" exited with failure: %s",
				countCommand,
				cmdStandardError,
			)))
		})
	})

})

func partitionCountCommand(diskNumber string) string {
	return fmt.Sprintf("powershell.exe Get-Disk -Number %s | Select -ExpandProperty NumberOfPartitions", diskNumber)
}
