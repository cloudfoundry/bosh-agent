package package_installer_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/system"

	"io"
	"strings"

	. "github.com/cloudfoundry/bosh-agent/bootstrapper/package_installer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("package_installer", mainDesc)

func mainDesc() {
	var (
		tmpDir           string
		tarball          io.Reader
		system           *fakeSystem
		packageInstaller PackageInstaller
	)

	BeforeEach(func() {
		tmpDir = "fake/tmp/dir"
		tarball = strings.NewReader("fake tarball")
		system = &fakeSystem{
			UntarExitStatus:     0,
			UntarCommandRun:     "the-untar-command",
			UntarError:          nil,
			RunScriptExitStatus: 0,
			RunScriptCommandRun: "the-install-script-command",
			RunScriptError:      nil,
			TempDirTempDir:      tmpDir,
			TempDirError:        nil,
			FileExistsBool:      true,
		}
	})

	Describe("#Install", func() {
		It("expands tarball from given stream into a temp dir and runs install.sh", func() {
			packageInstaller = New(system)
			installError := packageInstaller.Install(tarball)
			Expect(installError).ToNot(HaveOccurred())

			Expect(system.UntarTarball).To(Equal(tarball))
			Expect(system.UntarTargetDir).To(Equal(tmpDir))
			Expect(system.RunScriptScript).To(Equal("./install.sh"))
			Expect(system.RunScriptWorkingDir).To(Equal(tmpDir))
		})

		Context("when the tarball is invalid", func() {
			It("returns a non-system error with info", func() {
				system.UntarExitStatus = 100
				system.UntarCommandRun = "the-failing-untar-command"

				packageInstaller = New(system)

				installError := packageInstaller.Install(tarball)

				Expect(installError).To(HaveOccurred())
				expectedError := "`the-failing-untar-command` exited with 100"
				Expect(installError.Error()).To(Equal(expectedError))
				Expect(installError.SystemError()).To(BeFalse())
			})
		})

		Context("when the install script errors", func() {
			It("returns a non-system error with info", func() {
				system.RunScriptExitStatus = 100
				system.RunScriptCommandRun = "the-failing-install-command"

				packageInstaller = New(system)

				installError := packageInstaller.Install(tarball)

				Expect(installError).To(HaveOccurred())
				expectedError := "`the-failing-install-command` exited with 100"
				Expect(installError.Error()).To(Equal(expectedError))
				Expect(installError.SystemError()).To(BeFalse())
			})
		})

		Context("when the install script is not present", func() {
			It("returns a user error", func() {
				system.FileExistsBool = false

				packageInstaller = New(system)
				installError := packageInstaller.Install(tarball)

				Expect(installError).To(HaveOccurred())
				Expect(installError.Error()).To(Equal("No 'install.sh' script found"))
				Expect(installError.SystemError()).To(BeFalse())
			})
		})

		Context("when the system has errors", func() {
			It("returns a system error when we fail to run the tar command", func() {
				system.UntarError = errors.New("something went terribly wrong while untarring")

				packageInstaller = New(system)

				installError := packageInstaller.Install(tarball)

				Expect(installError).To(HaveOccurred())
				Expect(installError.Error()).To(Equal("something went terribly wrong while untarring"))
				Expect(installError.SystemError()).To(BeTrue())
			})

			It("returns a system error when we fail to run the install script", func() {
				system.RunScriptError = errors.New("something went terribly wrong while installing")

				packageInstaller = New(system)

				installError := packageInstaller.Install(tarball)

				Expect(installError).To(HaveOccurred())
				Expect(installError.Error()).To(Equal("something went terribly wrong while installing"))
				Expect(installError.SystemError()).To(BeTrue())
			})
		})
	})
}

type fakeSystem struct {
	UntarTarball    io.Reader
	UntarTargetDir  string
	UntarExitStatus int
	UntarCommandRun string
	UntarError      error

	RunScriptScript     string
	RunScriptWorkingDir string
	RunScriptExitStatus int
	RunScriptCommandRun string
	RunScriptError      error

	TempDirTempDir string
	TempDirError   error

	FileExistsBool bool
}

func (fake *fakeSystem) Untar(tarball io.Reader, targetDir string) (system.CommandResult, error) {
	fake.UntarTarball = tarball
	fake.UntarTargetDir = targetDir
	return system.CommandResult{ExitStatus: fake.UntarExitStatus, CommandRun: fake.UntarCommandRun}, fake.UntarError
}

func (fake *fakeSystem) RunScript(scriptPath string, workingDir string) (system.CommandResult, error) {
	fake.RunScriptScript = scriptPath
	fake.RunScriptWorkingDir = workingDir
	return system.CommandResult{ExitStatus: fake.RunScriptExitStatus, CommandRun: fake.RunScriptCommandRun}, fake.RunScriptError
}

func (fake *fakeSystem) FileExists(filePath string) bool {
	return fake.FileExistsBool
}

func (fake *fakeSystem) TempDir(dir string, prefix string) (string, error) {
	return fake.TempDirTempDir, fake.TempDirError
}
