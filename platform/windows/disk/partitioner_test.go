package disk_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/platform/windows/disk"
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

	Describe("GetFreeSpaceOnDisk", func() {
		It("returns the free space on disk", func() {
			expectedFreeSpace := 5 * 1024 * 1024 * 1024

			cmdRunner.AddCmdResult(
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `Get-Disk -Number 1 | Select-Object -ExpandProperty LargestFreeExtent`}, " "),
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
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `Get-Disk -Number 1 | Select-Object -ExpandProperty LargestFreeExtent`}, " "),
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
			expectedStdout := `Not a number
`

			cmdRunner.AddCmdResult(
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `Get-Disk -Number 1 | Select-Object -ExpandProperty LargestFreeExtent`}, " "),
				fakes.FakeCmdResult{
					Stdout: expectedStdout,
				},
			)

			_, err := partitioner.GetFreeSpaceOnDisk(diskNumber)
			Expect(err).To(MatchError(fmt.Sprintf(
				"Failed to convert output of \"%s\" command in to number. Output was: \"%s\"",
				`Get-Disk -Number 1 | Select-Object -ExpandProperty LargestFreeExtent`,
				strings.TrimSpace(expectedStdout),
			)))
		})

		It("rejects non-numeric disk identifiers", func() {
			_, err := partitioner.GetFreeSpaceOnDisk(`1;Invoke-Expression`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`GetFreeSpaceOnDisk: invalid disk number "1;Invoke-Expression"`))
		})
	})

	Describe("GetCountOnDisk", func() {
		It("returns number of partitions found on disk", func() {
			expectedPartitionCount := "2"

			cmdRunner.AddCmdResult(
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `Get-Disk -Number 1 | Select-Object -ExpandProperty NumberOfPartitions`}, " "),
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
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `Get-Disk -Number 1 | Select-Object -ExpandProperty NumberOfPartitions`}, " "),
				fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
			)

			_, err := partitioner.GetCountOnDisk(diskNumber)
			Expect(err).To(MatchError(fmt.Sprintf(
				"failed to get existing partition count for disk %s: %s",
				diskNumber,
				cmdRunnerError.Error(),
			)))
		})

		It("rejects non-numeric disk identifiers", func() {
			_, err := partitioner.GetCountOnDisk(`1;Invoke-Expression`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`GetCountOnDisk: invalid disk number "1;Invoke-Expression"`))
		})
	})

	Describe("InitializeDisk", func() {
		It("makes the request to initialize the given disk", func() {
			expected := []string{"-NoProfile", "-NonInteractive", "-Command", `Initialize-Disk -Number 1 -PartitionStyle GPT`}

			cmdRunner.AddCmdResult(strings.Join(expected, " "), fakes.FakeCmdResult{})

			err := partitioner.InitializeDisk(diskNumber)
			Expect(err).NotTo(HaveOccurred())
			Expect(cmdRunner.RunCommands).To(Equal([][]string{expected}))
		})

		It("when the command fails returns a wrapped error", func() {
			cmdRunnerError := errors.New("It went wrong")
			cmdRunner.AddCmdResult(
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `Initialize-Disk -Number 1 -PartitionStyle GPT`}, " "),
				fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
			)

			err := partitioner.InitializeDisk(diskNumber)
			Expect(err).To(MatchError(fmt.Sprintf("failed to initialize disk %s: %s", diskNumber, cmdRunnerError)))
		})

		It("rejects non-numeric disk identifiers", func() {
			err := partitioner.InitializeDisk(`1;Invoke-Expression`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`InitializeDisk: invalid disk number "1;Invoke-Expression"`))
		})
	})

	Describe("PartitionDisk", func() {
		It("makes the request to create a new parition and returns the generated partition number", func() {
			expectedPartitionNumber := "2"
			cmdRunner.AddCmdResult(
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `New-Partition -DiskNumber 1 -UseMaximumSize | Select-Object -ExpandProperty PartitionNumber`}, " "),
				fakes.FakeCmdResult{Stdout: fmt.Sprintf(`%s
`, expectedPartitionNumber)})

			partitionNumber, err := partitioner.PartitionDisk(diskNumber)
			Expect(err).NotTo(HaveOccurred())
			Expect(partitionNumber).To(Equal(expectedPartitionNumber))
			Expect(cmdRunner.RunCommands).To(Equal([][]string{{
				"-NoProfile", "-NonInteractive", "-Command", `New-Partition -DiskNumber 1 -UseMaximumSize | Select-Object -ExpandProperty PartitionNumber`,
			}}))
		})

		It("returns a wrapped error with no partition number when the command fails", func() {
			cmdRunnerError := errors.New("Failed to partition")
			cmdRunner.AddCmdResult(
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `New-Partition -DiskNumber 1 -UseMaximumSize | Select-Object -ExpandProperty PartitionNumber`}, " "),
				fakes.FakeCmdResult{Error: cmdRunnerError},
			)

			partitionNumber, err := partitioner.PartitionDisk(diskNumber)
			Expect(partitionNumber).To(BeEmpty())
			Expect(err).To(MatchError(
				fmt.Sprintf("failed to create partition on disk %s: %s", diskNumber, cmdRunnerError),
			))
		})

		It("rejects non-numeric disk identifiers", func() {
			_, err := partitioner.PartitionDisk(`1;Invoke-Expression`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`PartitionDisk: invalid disk number "1;Invoke-Expression"`))
		})
	})

	Describe("AssignDriveLetter", func() {
		var partitionNumber string

		BeforeEach(func() {
			partitionNumber = "2"
		})

		It("makes the request to add a partition path to given disk and partition returning the drive letter", func() {
			expectedDriveLetter := "G"
			cmdRunner.AddCmdResult(
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `Add-PartitionAccessPath -DiskNumber 1 -PartitionNumber 2 -AssignDriveLetter`}, " "),
				fakes.FakeCmdResult{},
			)
			cmdRunner.AddCmdResult(
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `Get-Partition -DiskNumber 1 -PartitionNumber 2 | Select-Object -ExpandProperty DriveLetter`}, " "),
				fakes.FakeCmdResult{Stdout: fmt.Sprintf(`%s
`, expectedDriveLetter)})

			driveLetter, err := partitioner.AssignDriveLetter(diskNumber, partitionNumber)
			Expect(err).NotTo(HaveOccurred())
			Expect(driveLetter).To(Equal(expectedDriveLetter))
			Expect(cmdRunner.RunCommands).To(Equal([][]string{
				{"-NoProfile", "-NonInteractive", "-Command", `Add-PartitionAccessPath -DiskNumber 1 -PartitionNumber 2 -AssignDriveLetter`},
				{"-NoProfile", "-NonInteractive", "-Command", `Get-Partition -DiskNumber 1 -PartitionNumber 2 | Select-Object -ExpandProperty DriveLetter`},
			}))
		})

		It("returns a wrapped error when the request to add a partition path fails", func() {
			addPartitionPathError := errors.New("failed to add path")
			cmdRunner.AddCmdResult(
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `Add-PartitionAccessPath -DiskNumber 1 -PartitionNumber 2 -AssignDriveLetter`}, " "),
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
			cmdRunner.AddCmdResult(
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `Add-PartitionAccessPath -DiskNumber 1 -PartitionNumber 2 -AssignDriveLetter`}, " "),
				fakes.FakeCmdResult{},
			)
			cmdRunner.AddCmdResult(
				strings.Join([]string{"-NoProfile", "-NonInteractive", "-Command", `Get-Partition -DiskNumber 1 -PartitionNumber 2 | Select-Object -ExpandProperty DriveLetter`}, " "),
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

		It("rejects non-numeric disk or partition identifiers", func() {
			_, err := partitioner.AssignDriveLetter(`1;Invoke-Expression`, partitionNumber)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`AssignDriveLetter: invalid disk number "1;Invoke-Expression"`))

			_, err = partitioner.AssignDriveLetter(diskNumber, `2;Invoke-Expression`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`AssignDriveLetter: invalid partition number "2;Invoke-Expression"`))
		})
	})
})
