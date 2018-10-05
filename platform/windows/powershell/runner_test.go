package powershell_test

import (
	"errors"
	"fmt"

	"strings"

	"os"
	"os/exec"

	"github.com/cloudfoundry/bosh-agent/platform/windows/powershell"
	"github.com/cloudfoundry/bosh-utils/system"
	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		baseCmdRunner    *fakes.FakeCmdRunner
		powershellRunner system.CmdRunner
		commandExitError = &exec.ExitError{
			ProcessState: &os.ProcessState{},
		}
	)

	BeforeEach(func() {
		baseCmdRunner = fakes.NewFakeCmdRunner()
	})

	Describe("RunCommand", func() {
		It("uses injected command runnner with powershell.exe prefix", func() {
			powershellCommand := "Do A Thing"
			powershellCommandArgs := strings.Split(powershellCommand, " ")
			expectedStdout := "Standard Out"
			expectedStderr := "Standard Err"
			expectedExitCode := 5
			expectedCommand := fmt.Sprintf("%s %s", powershell.Executable, powershellCommand)
			baseCmdRunner.AddCmdResult(
				expectedCommand,
				fakes.FakeCmdResult{
					Stdout:     expectedStdout,
					Stderr:     expectedStderr,
					ExitStatus: expectedExitCode,
				},
			)

			powershellRunner = &powershell.Runner{
				BaseCmdRunner: baseCmdRunner,
			}

			stdout, stderr, exitCode, err := powershellRunner.RunCommand(powershellCommandArgs[0], powershellCommandArgs[1:]...)
			Expect(stdout).To(Equal(expectedStdout))
			Expect(stderr).To(Equal(expectedStderr))
			Expect(exitCode).To(Equal(expectedExitCode))
			Expect(err).To(BeNil())
			Expect(baseCmdRunner.RunCommands).To(Equal([][]string{strings.Split(expectedCommand, " ")}))
		})

		It("when command fails to run returns a wrapped error", func() {
			powershellCommand := "Do A Thing"
			powershellCommandArgs := strings.Split(powershellCommand, " ")
			runnerError := errors.New("An error")
			expectedCommand := fmt.Sprintf("%s %s", powershell.Executable, powershellCommand)
			expectedExitCode := -1
			baseCmdRunner.AddCmdResult(
				expectedCommand,
				fakes.FakeCmdResult{
					ExitStatus: expectedExitCode,
					Error:      runnerError,
				},
			)

			powershellRunner = &powershell.Runner{
				BaseCmdRunner: baseCmdRunner,
			}

			_, _, exitCode, err := powershellRunner.RunCommand(powershellCommandArgs[0], powershellCommandArgs[1:]...)
			Expect(exitCode).To(Equal(expectedExitCode))
			Expect(err).To(MatchError(
				fmt.Sprintf("Failed to run command \"%s\": %s", expectedCommand, runnerError.Error()),
			))
			Expect(baseCmdRunner.RunCommands).To(Equal([][]string{strings.Split(expectedCommand, " ")}))
		})

		It("when command runs but returns non-zero exit code, returns the command standard error", func() {
			powershellCommand := "Do A Thing"
			powershellCommandArgs := strings.Split(powershellCommand, " ")
			expectedCommand := fmt.Sprintf("%s %s", powershell.Executable, powershellCommand)
			expectedExitCode := 197
			cmdStandardError := `At line:1 char:3
+ Do A Thing
+   ~
Missing statement body in do loop.
    + CategoryInfo          : ParserError: (:) [], ParentContainsErrorRecordException
    + FullyQualifiedErrorId : MissingLoopStatement
`

			baseCmdRunner.AddCmdResult(
				expectedCommand,
				fakes.FakeCmdResult{ExitStatus: expectedExitCode, Stderr: cmdStandardError, Error: commandExitError},
			)

			powershellRunner = &powershell.Runner{
				BaseCmdRunner: baseCmdRunner,
			}

			_, _, exitCode, err := powershellRunner.RunCommand(powershellCommandArgs[0], powershellCommandArgs[1:]...)
			Expect(err).To(MatchError(fmt.Sprintf(
				"Command \"%s\" exited with failure: %s",
				expectedCommand,
				cmdStandardError,
			)))
			Expect(exitCode).To(Equal(expectedExitCode))
		})
	})

	It("RunCommandQuietly uses injected command runner with powershell.exe prefix", func() {
		powershellCommandArgs := []string{"Do", "A", "Thing"}
		powershellCommand := strings.Join(powershellCommandArgs, " ")
		expectedCommand := fmt.Sprintf("%s %s", powershell.Executable, powershellCommand)

		powershellRunner = &powershell.Runner{
			BaseCmdRunner: baseCmdRunner,
		}

		powershellRunner.RunCommandQuietly(powershellCommandArgs[0], powershellCommandArgs[1:]...)
		Expect(baseCmdRunner.RunCommandsQuietly).To(Equal([][]string{strings.Split(expectedCommand, " ")}))
	})

	It("RunCommandWithInput uses injected command runner with powershell.exe prefix", func() {
		powershellCommandArgs := []string{"Do", "A", "Thing"}
		powershellCommand := strings.Join(powershellCommandArgs, " ")
		expectedCommand := fmt.Sprintf("%s %s", powershell.Executable, powershellCommand)

		powershellRunner = &powershell.Runner{
			BaseCmdRunner: baseCmdRunner,
		}

		input := "Some stdin"
		powershellRunner.RunCommandWithInput(input, powershellCommandArgs[0], powershellCommandArgs[1:]...)
		Expect(baseCmdRunner.RunCommandsWithInput).To(Equal(
			[][]string{append([]string{input}, strings.Split(expectedCommand, " ")...)},
		))
	})

	It("RunComplexCommand uses powershell.exe as cmd name, prefixing provided cmd name to args", func() {
		powershellCommandArgs := []string{"Do", "A", "Thing"}
		complexCommand := system.Command{
			Name: powershellCommandArgs[0],
			Args: powershellCommandArgs[1:],
		}
		expectedCommand := system.Command{
			Name: powershell.Executable,
			Args: powershellCommandArgs,
		}

		powershellRunner = &powershell.Runner{
			BaseCmdRunner: baseCmdRunner,
		}

		powershellRunner.RunComplexCommand(complexCommand)
		Expect(baseCmdRunner.RunComplexCommands).To(Equal([]system.Command{expectedCommand}))
	})

	It("RunComplexCommandAsync uses powershell.exe as cmd name, prefixing provided cmd name to args", func() {
		powershellCommandArgs := []string{"Do", "A", "Thing"}
		complexCommand := system.Command{
			Name: powershellCommandArgs[0],
			Args: powershellCommandArgs[1:],
		}
		expectedCommand := system.Command{
			Name: powershell.Executable,
			Args: powershellCommandArgs,
		}

		powershellRunner = &powershell.Runner{
			BaseCmdRunner: baseCmdRunner,
		}

		baseCmdRunner.AddProcess(
			strings.Join(append([]string{powershell.Executable}, powershellCommandArgs...), " "),
			&fakes.FakeProcess{},
		)

		powershellRunner.RunComplexCommandAsync(complexCommand)
		Expect(baseCmdRunner.RunComplexCommands).To(Equal([]system.Command{expectedCommand}))
	})

	Describe("CommandExists", func() {
		It("uses powershell's Get-Command output to determine commands existence", func() {
			command := "A-Powershell-Command"
			expectedGetCommand := fmt.Sprintf("%s Get-Command %s", powershell.Executable, command)
			baseCmdRunner.AddCmdResult(
				expectedGetCommand,
				fakes.FakeCmdResult{
					Stdout: `
CommandType     Name                                               Version    Source
-----------     ----                                               -------    ------
Function        A-Powershell-Command                               0.1        BOSH.Utils

`,
					ExitStatus: 0,
				},
			)

			powershellRunner = &powershell.Runner{
				BaseCmdRunner: baseCmdRunner,
			}

			found := powershellRunner.CommandExists(command)
			Expect(found).To(BeTrue())
			Expect(baseCmdRunner.RunCommands).To(Equal([][]string{strings.Split(expectedGetCommand, " ")}))
		})

		It("returns false when Get-Command fails", func() {
			command := "Another-Powershell-Command"
			expectedGetCommand := fmt.Sprintf("%s Get-Command %s", powershell.Executable, command)
			baseCmdRunner.AddCmdResult(
				expectedGetCommand,
				fakes.FakeCmdResult{
					Stderr: `get-command : The term 'Another-Powershell-Command' is not recognized as the name of a cmdlet, function, script file, or operable program. Check the spelling of the name, or if
a path was included, verify that the path is correct and try again.
At line:1 char:1
+ get-command Another-Powershell-Command
+ ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : ObjectNotFound: (Another-Powershell-Command:String) [Get-Command], CommandNotFoundException
    + FullyQualifiedErrorId : CommandNotFoundException,Microsoft.PowerShell.Commands.GetCommandCommand
`,
					ExitStatus: 1,
				},
			)

			powershellRunner = &powershell.Runner{
				BaseCmdRunner: baseCmdRunner,
			}

			found := powershellRunner.CommandExists(command)
			Expect(found).To(BeFalse())
		})
	})
})
