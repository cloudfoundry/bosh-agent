package scriptrunner_test

import (
	"errors"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/scriptrunner"
	fakesys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system/fakes"
	"path/filepath"
)

var _ = Describe("GenericScript", func() {
	var (
		fs            *fakesys.FakeFileSystem
		cmdRunner     *fakesys.FakeCmdRunner
		genericScript scriptrunner.GenericScript
		stdoutLogPath string
		stderrLogPath string
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		cmdRunner = fakesys.NewFakeCmdRunner()
		stdoutLogPath = filepath.Join("base", "stdout", "logdir", "stdout.log")
		stderrLogPath = filepath.Join("base", "stderr", "logdir", "stderr.log")
		genericScript = scriptrunner.NewScript(
			"my-tag",
			fs,
			cmdRunner,
			"/path-to-script",
			stdoutLogPath,
			stderrLogPath)
	})

	Describe("RunCommand", func() {

		It("executes given command", func() {
			runScriptResult := genericScript.Run()

			Expect(runScriptResult.Tag).To(Equal("my-tag"))
			Expect(runScriptResult.Error).To(BeNil())
		})

		It("returns an error if it fails to create logs directory", func() {
			fs.MkdirAllError = errors.New("fake-mkdir-all-error")

			runScriptResult := genericScript.Run()

			Expect(runScriptResult.Tag).To(Equal("my-tag"))
			Expect(runScriptResult.Error.Error()).To(Equal("fake-mkdir-all-error"))
		})

		It("returns an error if it fails to open stdout/stderr log file", func() {
			fs.OpenFileErr = errors.New("fake-open-file-error")

			runScriptResult := genericScript.Run()

			Expect(runScriptResult.Tag).To(Equal("my-tag"))
			Expect(runScriptResult.Error.Error()).To(Equal("fake-open-file-error"))
		})

		Context("when command succeeds", func() {

			BeforeEach(func() {
				cmdRunner.AddCmdResult("/path-to-script", fakesys.FakeCmdResult{
					Stdout:     "fake-stdout",
					Stderr:     "fake-stderr",
					ExitStatus: 0,
					Error:      nil,
				})
			})

			It("saves stdout/stderr to log file", func() {
				runScriptResult := genericScript.Run()

				Expect(runScriptResult.Tag).To(Equal("my-tag"))
				Expect(runScriptResult.Error).To(BeNil())

				Expect(fs.FileExists(stdoutLogPath)).To(BeTrue())
				Expect(fs.FileExists(stderrLogPath)).To(BeTrue())

				stdout, err := fs.ReadFileString(stdoutLogPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(stdout).To(Equal("fake-stdout"))

				stderr, err := fs.ReadFileString(stderrLogPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(stderr).To(Equal("fake-stderr"))
			})
		})

		Context("when command fails", func() {

			BeforeEach(func() {
				cmdRunner.AddCmdResult("/path-to-script", fakesys.FakeCmdResult{
					Stdout:     "fake-stdout",
					Stderr:     "fake-stderr",
					ExitStatus: 1,
					Error:      errors.New("fake-command-error"),
				})
			})

			It("saves stdout/stderr to log file", func() {
				runScriptResult := genericScript.Run()

				Expect(runScriptResult.Tag).To(Equal("my-tag"))
				Expect(runScriptResult.Error.Error()).To(Equal("fake-command-error"))

				Expect(fs.FileExists(stdoutLogPath)).To(BeTrue())
				Expect(fs.FileExists(stderrLogPath)).To(BeTrue())

				stdout, err := fs.ReadFileString(stdoutLogPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(stdout).To(Equal("fake-stdout"))

				stderr, err := fs.ReadFileString(stderrLogPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(stderr).To(Equal("fake-stderr"))
			})
		})
	})
})
