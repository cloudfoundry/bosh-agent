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

		It("when the isLinked command fails to run, returns a wrapped error", func() {
			cmdRunnerError := errors.New("It went wrong")
			linkTargetCommand := findItemTargetCommand(location)
			cmdRunner.AddCmdResult(
				linkTargetCommand,
				fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
			)

			_, err := linker.LinkTarget(location)
			Expect(err).To(MatchError(fmt.Sprintf(
				"Failed to run command \"%s\": It went wrong",
				linkTargetCommand,
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

		It("when the command fails to run, returns a wrapped error", func() {
			cmdRunnerError := errors.New("It went wrong")
			linkCommand := createLinkCommand(location, target)

			cmdRunner.AddCmdResult(
				linkCommand,
				fakes.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError},
			)

			err := linker.Link(location, target)
			Expect(err).To(MatchError(fmt.Sprintf(
				"Failed to run command \"%s\": It went wrong",
				linkCommand,
			)))
		})

		It("when the command syntax is incorrect, it returns an error including syntax help message", func() {
			linkCommand := createLinkCommand("", target)
			cmdStderr := `The syntax of the command is incorrect.
Creates a symbolic link.

MKLINK [[/D] | [/H] | [/J]] Link Target

				/D			Creates a directory symbolic link.  Default is a file
								symbolic link.
				/H      Creates a hard link instead of a symbolic link.
				/J      Creates a Directory Junction.
				Link    Specifies the new symbolic link name.
				Target  Specifies the path (relative or absolute) that the new link
								refers to.`

			cmdRunner.AddCmdResult(
				linkCommand,
				fakes.FakeCmdResult{
					ExitStatus: 1,
					Stderr:     cmdStderr,
					Error:      commandExitError,
				},
			)

			err := linker.Link("", target)
			Expect(err).To(MatchError(fmt.Sprintf(
				"Command \"%s\" exited with failure: %s",
				linkCommand,
				cmdStderr,
			)))
		})
	})
})

func findItemTargetCommand(location string) string {
	return fmt.Sprintf(
		"powershell.exe Get-Item %s -ErrorAction Ignore | Select -ExpandProperty Target -ErrorAction Ignore",
		location,
	)
}

func createLinkCommand(location, driveLetter string) string {
	return fmt.Sprintf("cmd.exe /c mklink /d %s %s", location, driveLetter)
}
