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

var _ = Describe("Protector", func() {
	var (
		powershellRunner *fakes.FakeCmdRunner
		protector        *disk.Protector
	)

	BeforeEach(func() {
		powershellRunner = fakes.NewFakeCmdRunner()
		protector = &disk.Protector{
			Runner: powershellRunner,
		}
	})

	Describe("CommandExists", func() {
		It("calls CommandExists of powershell runner, returns the result when true", func() {
			powershellRunner.CommandExistsValue = true

			protector := &disk.Protector{
				Runner: powershellRunner,
			}

			Expect(protector.CommandExists()).To(BeTrue())
		})

		It("calls CommandExists of powershell runner, returns the result when false", func() {
			powershellRunner := fakes.NewFakeCmdRunner()
			powershellRunner.CommandExistsValue = false

			protector := &disk.Protector{
				Runner: powershellRunner,
			}

			Expect(protector.CommandExists()).To(BeFalse())
		})
	})

	Describe("ProtectPath", func() {
		var (
			expectedDataDir,
			providedDataDir,
			expectedCommand string
		)

		BeforeEach(func() {
			expectedDataDir = `C:\\var\\data`
			providedDataDir = fmt.Sprintf("%s\\", expectedDataDir)
			expectedCommand = fmt.Sprintf(`%s '%s'`, disk.ProtectCmdlet, expectedDataDir)
		})

		It("calls Protect-Path cmdlet with trailing slashes removed", func() {
			powershellRunner.AddCmdResult(expectedCommand, fakes.FakeCmdResult{})

			err := protector.ProtectPath(providedDataDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(powershellRunner.RunCommands).To(Equal([][]string{strings.Split(expectedCommand, " ")}))
		})

		It("returns a wrapped error when protect-path command fails", func() {
			cmdRunnerError := errors.New("command runner failed")

			powershellRunner.AddCmdResult(expectedCommand, fakes.FakeCmdResult{Error: cmdRunnerError})

			err := protector.ProtectPath(providedDataDir)
			Expect(err).To(MatchError(fmt.Sprintf("failed to protect '%s': %s", providedDataDir, cmdRunnerError)))
		})
	})
})
