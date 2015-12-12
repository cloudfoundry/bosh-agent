// +build windows

package system_test

import (
	"errors"

	. "github.com/cloudfoundry/bosh-utils/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-utils/internal/github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("PSRunner", func() {
	var (
		fs     FileSystem
		runner PSRunner
		logger boshlog.Logger
	)

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)
		fs = NewOsFileSystem(logger)
		runner = NewConcretePSRunner(fs, logger)
	})

	Describe("RunCommand", func() {
		It("runs a successful Powershell command and doesnt return an error", func() {
			command := PSCommand{
				Script: `
Write-Output stdout
[Console]::Error.WriteLine('stderr')
`,
			}
			stdout, stderr, err := runner.RunCommand(command)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(Equal("stdout\r\n"))
			Expect(stderr).To(Equal("stderr\r\n"))
		})

		It("runs a failing Powershell command and returns error", func() {
			command := PSCommand{
				Script: `
Write-Output stdout
[Console]::Error.WriteLine('stderr')
Exit 10
`,
			}
			stdout, stderr, err := runner.RunCommand(command)
			Expect(err).To(HaveOccurred())
			Expect(stdout).To(Equal("stdout\r\n"))
			Expect(stderr).To(Equal("stderr\r\n"))
		})

		It("runs a successful single line command", func() {
			command := PSCommand{
				Script: `Write-Output stdout; [Console]::Error.WriteLine('stderr')`,
			}
			stdout, stderr, err := runner.RunCommand(command)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(Equal("stdout\r\n"))
			Expect(stderr).To(Equal("stderr\r\n"))
		})

		It("does not error if script is empty", func() {
			command := PSCommand{
				Script: "",
			}
			stdout, stderr, err := runner.RunCommand(command)
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(Equal(""))
			Expect(stderr).To(Equal(""))
		})

		Context("filesystem errors", func() {
			var fs *fakesys.FakeFileSystem

			BeforeEach(func() {
				fs = fakesys.NewFakeFileSystem()
				runner = NewConcretePSRunner(fs, logger)
			})

			Context("when creating Tempfile fails", func() {
				It("errors out", func() {
					fs.TempFileError = errors.New("boo")

					_, _, err := runner.RunCommand(PSCommand{})
					Expect(err.Error()).To(Equal("Creating tempfile: boo"))
				})
			})

			Context("when writing to the Tempfile fails", func() {
				It("errors out", func() {
					tempfile := fakesys.NewFakeFile("path", fs)
					fs.ReturnTempFile = tempfile
					tempfile.WriteErr = errors.New("foo")

					_, _, err := runner.RunCommand(PSCommand{})
					Expect(err.Error()).To(Equal("Writing to tempfile: foo"))
				})
			})

			Context("when closing Tempfile fails", func() {
				It("errors out", func() {
					tempfile := fakesys.NewFakeFile("path", fs)
					fs.ReturnTempFile = tempfile
					tempfile.CloseErr = errors.New("oh noes")

					_, _, err := runner.RunCommand(PSCommand{})
					Expect(err.Error()).To(Equal("Closing tempfile: oh noes"))
				})
			})

			Context("when renaming Tempfile fails", func() {
				It("errors out", func() {
					tempfile := fakesys.NewFakeFile("path", fs)
					fs.ReturnTempFile = tempfile
					fs.RenameError = errors.New("sigh")

					_, _, err := runner.RunCommand(PSCommand{})
					Expect(err.Error()).To(Equal("Renaming tempfile: sigh"))
				})
			})
		})
	})
})
