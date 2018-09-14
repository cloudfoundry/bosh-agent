package disk_test

import (
	"errors"
	"fmt"

	"strings"

	"github.com/cloudfoundry/bosh-agent/platform/windows/disk"
	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Partitioner", func() {
	const cmdStandardError = `Get-Disk : No MSFT_Disk objects found with property 'Number' equal to '0'.
Verify the value of the property and retry.
At line:1 char:1
+ Get-Disk 0
+ ~~~~~~~~~~
    + CategoryInfo          : ObjectNotFound: (0:UInt32) [Get-Disk], CimJobExc
   eption
    + FullyQualifiedErrorId : CmdletizationQuery_NotFound_Number,Get-Disk
`

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

	Describe("GetFreeSpaceOnDisk", func() {
		It("returns the free space on disk", func() {
			expectedFreeSpace := 5 * 1024 * 1024 * 1024

			cmdRunner.AddCmdResult(
				partitionFreeSpaceCommand(diskNumber),
				fakes.FakeCmdResult{
					Stdout: fmt.Sprintf(`%d
`, expectedFreeSpace),
				},
			)

			freeSpace, err := partitioner.GetFreeSpaceOnDisk(diskNumber)
			Expect(err).NotTo(HaveOccurred())
			Expect(freeSpace).To(Equal(expectedFreeSpace))

		})

		It("when the command fails returns a wrapped error", func() {
			cmdRunnerError := errors.New("It went wrong")
			cmdRunner.AddCmdResult(
				partitionFreeSpaceCommand(diskNumber),
				fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
			)

			_, err := partitioner.GetFreeSpaceOnDisk(diskNumber)
			Expect(err).To(MatchError(fmt.Sprintf(
				"failed to find free space on disk %s: %s",
				diskNumber,
				cmdRunnerError.Error(),
			)))
		})

		It("when response of command is not a number, returns an informative error", func() {
			freeSpaceCommand := partitionFreeSpaceCommand(diskNumber)
			expectedStdout := `Not a number
`

			cmdRunner.AddCmdResult(
				freeSpaceCommand,
				fakes.FakeCmdResult{
					Stdout: expectedStdout,
				},
			)

			_, err := partitioner.GetFreeSpaceOnDisk(diskNumber)
			Expect(err).To(MatchError(fmt.Sprintf(
				"Failed to convert output of \"%s\" command in to number. Output was: \"%s\"",
				freeSpaceCommand,
				strings.TrimSpace(expectedStdout),
			)))
		})
	})

	Describe("GetCountOnDisk", func() {
		It("returns number of partitions found on disk", func() {
			expectedPartitionCount := "2"

			cmdRunner.AddCmdResult(
				partitionCountCommand(diskNumber),
				fakes.FakeCmdResult{
					Stdout: fmt.Sprintf(`%s
`, expectedPartitionCount),
				},
			)

			partitionCount, err := partitioner.GetCountOnDisk(diskNumber)
			Expect(err).NotTo(HaveOccurred())
			Expect(partitionCount).To(Equal(expectedPartitionCount))
		})

		It("when the command fails returns a wrapped error", func() {
			cmdRunnerError := errors.New("It went wrong")
			cmdRunner.AddCmdResult(
				partitionCountCommand(diskNumber),
				fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
			)

			_, err := partitioner.GetCountOnDisk(diskNumber)
			Expect(err).To(MatchError(fmt.Sprintf(
				"failed to get existing partition count for disk %s: %s",
				diskNumber,
				cmdRunnerError.Error(),
			)))
		})
	})

	Describe("InitializeDisk", func() {
		It("makes the request to initialize the given disk", func() {
			expectedCommand := initializeDiskCommand(diskNumber)

			cmdRunner.AddCmdResult(expectedCommand, fakes.FakeCmdResult{})

			err := partitioner.InitializeDisk(diskNumber)
			Expect(err).NotTo(HaveOccurred())
			Expect(cmdRunner.RunCommands).To(Equal([][]string{strings.Split(expectedCommand, " ")}))
		})

		It("when the command fails returns a wrapped error", func() {
			cmdRunnerError := errors.New("It went wrong")
			cmdRunner.AddCmdResult(
				initializeDiskCommand(diskNumber),
				fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
			)

			err := partitioner.InitializeDisk(diskNumber)
			Expect(err).To(MatchError(fmt.Sprintf("failed to initialize disk %s: %s", diskNumber, cmdRunnerError)))
		})
	})

	Describe("PartitionDisk", func() {
		It("makes the request to create a new parition and returns the generated partition number", func() {
			expectedCommand := partitionDiskCommand(diskNumber)
			expectedPartitionNumber := "2"
			cmdRunner.AddCmdResult(expectedCommand, fakes.FakeCmdResult{Stdout: fmt.Sprintf(`%s
`, expectedPartitionNumber)})

			partitionNumber, err := partitioner.PartitionDisk(diskNumber)
			Expect(err).NotTo(HaveOccurred())
			Expect(partitionNumber).To(Equal(expectedPartitionNumber))
			Expect(cmdRunner.RunCommands).To(Equal([][]string{strings.Split(expectedCommand, " ")}))
		})

		It("returns a wrapped error with no partition number when the command fails", func() {
			cmdRunnerError := errors.New("Failed to partition")
			cmdRunner.AddCmdResult(
				partitionDiskCommand(diskNumber),
				fakes.FakeCmdResult{Error: cmdRunnerError},
			)

			partitionNumber, err := partitioner.PartitionDisk(diskNumber)
			Expect(partitionNumber).To(BeEmpty())
			Expect(err).To(MatchError(
				fmt.Sprintf("failed to create partition on disk %s: %s", diskNumber, cmdRunnerError),
			))
		})
	})

	Describe("AssignDriveLetter", func() {
		var partitionNumber string

		BeforeEach(func() {
			partitionNumber = "2"
		})

		It("makes the request to add a partition path to given disk and partition returning the drive letter", func() {
			expectedDriveLetter := "G"
			addPartitionPathCommand := addPartitionAccessPathCommand(diskNumber, partitionNumber)
			getDriveCommand := getDriveLetterCommand(diskNumber, partitionNumber)
			cmdRunner.AddCmdResult(addPartitionPathCommand, fakes.FakeCmdResult{})
			cmdRunner.AddCmdResult(getDriveCommand, fakes.FakeCmdResult{Stdout: fmt.Sprintf(`%s
`, expectedDriveLetter)})

			driveLetter, err := partitioner.AssignDriveLetter(diskNumber, partitionNumber)
			Expect(err).NotTo(HaveOccurred())
			Expect(driveLetter).To(Equal(expectedDriveLetter))
			Expect(cmdRunner.RunCommands).To(Equal([][]string{
				strings.Split(addPartitionPathCommand, " "),
				strings.Split(getDriveCommand, " "),
			}))
		})

		It("returns a wrapped error when the request to add a partition path fails", func() {
			addPartitionPathError := errors.New("failed to add path")
			cmdRunner.AddCmdResult(
				addPartitionAccessPathCommand(diskNumber, partitionNumber),
				fakes.FakeCmdResult{Error: addPartitionPathError},
			)

			driveLetter, err := partitioner.AssignDriveLetter(diskNumber, partitionNumber)
			Expect(err).To(MatchError(fmt.Sprintf(
				"failed to add partition access path to partition %s on disk %s: %s",
				partitionNumber,
				diskNumber,
				addPartitionPathError,
			)))
			Expect(driveLetter).To(Equal(""))
		})

		It("return a wrapped error when the request to discover the drive letter fails", func() {
			getDriveLetterError := errors.New("failed to discover drive letter")
			cmdRunner.AddCmdResult(addPartitionAccessPathCommand(diskNumber, partitionNumber), fakes.FakeCmdResult{})
			cmdRunner.AddCmdResult(
				getDriveLetterCommand(diskNumber, partitionNumber),
				fakes.FakeCmdResult{Error: getDriveLetterError},
			)

			driveLetter, err := partitioner.AssignDriveLetter(diskNumber, partitionNumber)
			Expect(err).To(MatchError(fmt.Sprintf(
				"failed to find drive letter for partition %s on disk %s: %s",
				partitionNumber,
				diskNumber,
				getDriveLetterError,
			)))
			Expect(driveLetter).To(Equal(""))
		})
	})
})

func partitionCountCommand(diskNumber string) string {
	return fmt.Sprintf("Get-Disk -Number %s | Select -ExpandProperty NumberOfPartitions", diskNumber)
}

func partitionFreeSpaceCommand(diskNumber string) string {
	return fmt.Sprintf("Get-Disk %s | Select -ExpandProperty LargestFreeExtent", diskNumber)
}

func initializeDiskCommand(diskNumber string) string {
	return fmt.Sprintf("Initialize-Disk -Number %s -PartitionStyle GPT", diskNumber)
}

func partitionDiskCommand(diskNumber string) string {
	return fmt.Sprintf(
		"New-Partition -DiskNumber %s -UseMaximumSize | Select -ExpandProperty PartitionNumber",
		diskNumber,
	)
}

func addPartitionAccessPathCommand(diskNumber, partitionNumber string) string {
	return fmt.Sprintf(
		"Add-PartitionAccessPath -DiskNumber %s -PartitionNumber %s -AssignDriveLetter",
		diskNumber,
		partitionNumber,
	)
}

func getDriveLetterCommand(diskNumber, partitionNumber string) string {
	return fmt.Sprintf(
		"Get-Partition -DiskNumber %s -PartitionNumber %s | Select -ExpandProperty DriveLetter",
		diskNumber,
		partitionNumber,
	)
}
