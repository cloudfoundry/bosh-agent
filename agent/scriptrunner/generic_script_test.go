package scriptrunner_test

import (
	"errors"
	"time"

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
			resultChannel := make(chan scriptrunner.RunScriptResult)
			go genericScript.Run(resultChannel)

			var receivedTagName string
			var returnedError error

			select {
			case runScriptResult := <-resultChannel:
				receivedTagName = runScriptResult.Tag
				returnedError = runScriptResult.Error
			case <-time.After(time.Second * 2):
				//If it went here , it will fail
			}

			Expect(receivedTagName).To(Equal("my-tag"))
			Expect(returnedError).To(BeNil())
		})

		It("returns an error if it fails to create logs directory", func() {
			fs.MkdirAllError = errors.New("fake-mkdir-all-error")

			resultChannel := make(chan scriptrunner.RunScriptResult)
			go genericScript.Run(resultChannel)

			var receivedTagName string
			var returnedError error

			select {
			case runScriptResult := <-resultChannel:
				receivedTagName = runScriptResult.Tag
				returnedError = runScriptResult.Error
			case <-time.After(time.Second * 2):
				//If it went here , it will fail
			}

			Expect(receivedTagName).To(Equal("my-tag"))
			Expect(returnedError.Error()).To(Equal("fake-mkdir-all-error"))
		})

		It("returns an error if it fails to open stdout/stderr log file", func() {
			fs.OpenFileErr = errors.New("fake-open-file-error")

			resultChannel := make(chan scriptrunner.RunScriptResult)
			go genericScript.Run(resultChannel)

			var receivedTagName string
			var returnedError error

			select {
			case runScriptResult := <-resultChannel:
				receivedTagName = runScriptResult.Tag
				returnedError = runScriptResult.Error
			case <-time.After(time.Second * 2):
				//If it went here , it will fail
			}

			Expect(receivedTagName).To(Equal("my-tag"))
			Expect(returnedError.Error()).To(Equal("fake-open-file-error"))
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
				resultChannel := make(chan scriptrunner.RunScriptResult)
				go genericScript.Run(resultChannel)

				var receivedTagName string
				var returnedError error

				select {
				case runScriptResult := <-resultChannel:
					receivedTagName = runScriptResult.Tag
					returnedError = runScriptResult.Error
				case <-time.After(time.Second * 2):
					//If it went here , it will fail
				}

				Expect(receivedTagName).To(Equal("my-tag"))
				Expect(returnedError).To(BeNil())

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
				resultChannel := make(chan scriptrunner.RunScriptResult)
				go genericScript.Run(resultChannel)

				var receivedTagName string
				var returnedError error

				select {
				case runScriptResult := <-resultChannel:
					receivedTagName = runScriptResult.Tag
					returnedError = runScriptResult.Error
				case <-time.After(time.Second * 2):
					//If it went here , it will fail
				}

				Expect(receivedTagName).To(Equal("my-tag"))
				Expect(returnedError.Error()).To(Equal("fake-command-error"))

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
