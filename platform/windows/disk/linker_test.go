package disk_test

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry/bosh-agent/platform/windows/disk"
	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const newLine = `
`

var _ = Describe("Linker", func() {
	var (
		cmdRunner *fakes.FakeCmdRunner
		linker    *disk.Linker
		location  string
	)

	BeforeEach(func() {
		cmdRunner = fakes.NewFakeCmdRunner()
		linker = &disk.Linker{
			Runner: cmdRunner,
		}
		location = `C:\my\location`
	})

	It("returns the linked destination when the link exists", func() {
		expectedTarget := `D:\`
		cmdRunner.AddCmdResult(
			findItemTargetCommand(location),
			fakes.FakeCmdResult{Stdout: fmt.Sprintf("%s%s", expectedTarget, newLine)},
		)

		target, err := linker.IsLinked(location)

		Expect(err).NotTo(HaveOccurred())
		Expect(target).To(Equal(expectedTarget))
	})

	It("when the isLinked command fails to run, returns a wrapped error", func() {
		cmdRunnerError := errors.New("It went wrong")
		cmdRunner.AddCmdResult(
			findItemTargetCommand(location),
			fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
		)

		_, err := linker.IsLinked(location)
		Expect(err).To(MatchError(fmt.Sprintf(
			"Failed to run command \"%s\": It went wrong",
			findItemTargetCommand(location),
		)))
	})
})

func findItemTargetCommand(location string) string {
	return fmt.Sprintf(
		"powershell.exe Get-Item %s -ErrorAction Ignore | Select -ExpandProperty Target -ErrorAction Ignore",
		location,
	)
}
