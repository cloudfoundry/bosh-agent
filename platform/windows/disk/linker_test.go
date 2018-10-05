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

	Describe("LinkTarget", func() {
		It("returns the linked destination when the link exists", func() {
			expectedTarget := `D:\`
			cmdRunner.AddCmdResult(
				findItemTargetCommand(location),
				fakes.FakeCmdResult{Stdout: fmt.Sprintf("%s%s", expectedTarget, newLine)},
			)

			target, err := linker.LinkTarget(location)

			Expect(err).NotTo(HaveOccurred())
			Expect(target).To(Equal(expectedTarget))
		})

		It("returns nothing when the link does not exist", func() {
			expectedTarget := ""
			badLocation := `c:\road\to\nowhere`

			cmdRunner.AddCmdResult(
				findItemTargetCommand(badLocation),
				fakes.FakeCmdResult{ExitStatus: 1, Stdout: fmt.Sprintf("%s%s", expectedTarget, newLine)},
			)

			target, err := linker.LinkTarget(badLocation)

			Expect(err).NotTo(HaveOccurred())
			Expect(target).To(Equal(expectedTarget))
		})

		It("when the LinkTarget command fails to run, returns a wrapped error", func() {
			cmdRunnerError := errors.New("It went wrong")
			linkTargetCommand := findItemTargetCommand(location)
			cmdRunner.AddCmdResult(
				linkTargetCommand,
				fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
			)

			_, err := linker.LinkTarget(location)
			Expect(err).To(MatchError(fmt.Sprintf(
				"failed to check for existing symbolic link: %s",
				cmdRunnerError.Error(),
			)))
		})
	})

	Describe("Link", func() {
		var target string

		BeforeEach(func() {
			target = "F:"
		})

		It("makes the request to create symlink", func() {
			expectedCommand := createLinkCommand(location, target)

			cmdRunner.AddCmdResult(expectedCommand, fakes.FakeCmdResult{})

			err := linker.Link(location, target)
			Expect(err).NotTo(HaveOccurred())

			Expect(cmdRunner.RunCommands[0]).To(Equal(strings.Split(expectedCommand, " ")))
		})

		It("when the link command fails returns a wrapped error", func() {
			cmdRunnerError := errors.New("It went wrong")
			cmdRunner.AddCmdResult(
				createLinkCommand(location, target),
				fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
			)

			err := linker.Link(location, target)
			Expect(err).To(MatchError(fmt.Sprintf("failed to create symbolic link: %s", cmdRunnerError.Error())))
		})
	})
})

func findItemTargetCommand(location string) string {
	return fmt.Sprintf(
		"Get-Item %s -ErrorAction Ignore | Select -ExpandProperty Target -ErrorAction Ignore",
		location,
	)
}

func createLinkCommand(location, driveLetter string) string {
	return fmt.Sprintf("cmd.exe /c mklink /d %s %s", location, driveLetter)
}
