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

var _ = Describe("Formatter", func() {
	var (
		formatter *disk.Formatter
		cmdRunner *fakes.FakeCmdRunner
	)

	BeforeEach(func() {
		cmdRunner = fakes.NewFakeCmdRunner()

		formatter = &disk.Formatter{
			Runner: cmdRunner,
		}
	})

	It("Sends a format command to cmdrunner", func() {
		expectedCommand := formatVolumeCommand("1", "2")

		cmdRunner.AddCmdResult(expectedCommand, fakes.FakeCmdResult{ExitStatus: 0})

		err := formatter.Format("1", "2")

		Expect(err).NotTo(HaveOccurred())

		Expect(len(cmdRunner.RunCommands)).To(Equal(1))
		Expect(cmdRunner.RunCommands[0]).To(Equal(strings.Split(expectedCommand, " ")))
	})

	It("when the format command fails returns a wrapped error", func() {
		cmdRunnerError := errors.New("It went wrong")
		cmdRunner.AddCmdResult(
			formatVolumeCommand("1", "2"),
			fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
		)

		err := formatter.Format("1", "2")
		Expect(err).To(MatchError(fmt.Sprintf("failed to format volume: %s", cmdRunnerError.Error())))
	})
})

func formatVolumeCommand(diskNumber, partitionNumber string) string {
	return fmt.Sprintf(
		"Get-Partition -DiskNumber %s -PartitionNumber %s | Format-Volume -FileSystem NTFS -Confirm:$false",
		diskNumber, partitionNumber,
	)
}
