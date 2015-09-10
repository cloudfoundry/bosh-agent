package scriptrunner_test

import (
	"errors"
	"time"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/scriptrunner"
	fakesys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("GenericScript", func() {
	var (
		fs            *fakesys.FakeFileSystem
		cmdRunner     *fakesys.FakeCmdRunner
		genericScript scriptrunner.GenericScript
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		cmdRunner = fakesys.NewFakeCmdRunner()
		genericScript = scriptrunner.NewScript(fs, cmdRunner, "/path-to-script", "/fake-base-dir/fake-log-path/script-name", "fake-job-name")
	})

	Describe("RunCommand", func() {

		It("executes given command", func() {
			errorChan := make(chan scriptrunner.RunScriptResult)
			doneChan := make(chan scriptrunner.RunScriptResult)

			go genericScript.Run(errorChan, doneChan)

			var passedJobName string
			var returnedError error

			select {
			case runScriptResult := <-doneChan:
				passedJobName = runScriptResult.JobName
				returnedError = runScriptResult.Error
			case <-time.After(time.Second * 2):
				//If it went here , it will fail
			}

			Expect(passedJobName).To(Equal("fake-job-name"))
			Expect(returnedError).To(BeNil())
		})

		It("returns an error if it fails to create logs directory", func() {
			fs.MkdirAllError = errors.New("fake-mkdir-all-error")

			errorChan := make(chan scriptrunner.RunScriptResult)
			doneChan := make(chan scriptrunner.RunScriptResult)

			go genericScript.Run(errorChan, doneChan)

			var failedJobName string
			var returnedError error

			select {
			case failedScript := <-errorChan:
				failedJobName = failedScript.JobName
				returnedError = failedScript.Error
			case <-time.After(time.Second * 2):
				//If it went here , it will fail
			}

			Expect(failedJobName).To(Equal("fake-job-name"))
			Expect(returnedError.Error()).To(Equal("fake-mkdir-all-error"))
		})

		It("returns an error if it fails to open stdout/stderr log file", func() {
			fs.OpenFileErr = errors.New("fake-open-file-error")

			errorChan := make(chan scriptrunner.RunScriptResult)
			doneChan := make(chan scriptrunner.RunScriptResult)

			go genericScript.Run(errorChan, doneChan)

			var failedJobName string
			var returnedError error

			select {
			case failedScript := <-errorChan:
				failedJobName = failedScript.JobName
				returnedError = failedScript.Error
			case <-time.After(time.Second * 2):
				//If it went here , it will fail
			}

			Expect(failedJobName).To(Equal("fake-job-name"))
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
				errorChan := make(chan scriptrunner.RunScriptResult)
				doneChan := make(chan scriptrunner.RunScriptResult)

				go genericScript.Run(errorChan, doneChan)

				var passedJobName string
				var returnedError error

				select {
				case runScriptResult := <-doneChan:
					passedJobName = runScriptResult.JobName
					returnedError = runScriptResult.Error
				case <-time.After(time.Second * 2):
					//If it went here , it will fail
				}

				Expect(passedJobName).To(Equal("fake-job-name"))
				Expect(returnedError).To(BeNil())

				Expect(fs.FileExists("/fake-base-dir/fake-log-path/script-name.stdout.log")).To(BeTrue())
				Expect(fs.FileExists("/fake-base-dir/fake-log-path/script-name.stderr.log")).To(BeTrue())

				stdout, err := fs.ReadFileString("/fake-base-dir/fake-log-path/script-name.stdout.log")
				Expect(err).ToNot(HaveOccurred())
				Expect(stdout).To(Equal("fake-stdout"))

				stderr, err := fs.ReadFileString("/fake-base-dir/fake-log-path/script-name.stderr.log")
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
				errorChan := make(chan scriptrunner.RunScriptResult)
				doneChan := make(chan scriptrunner.RunScriptResult)

				go genericScript.Run(errorChan, doneChan)

				var failedJobName string
				var returnedError error

				select {
				case runScriptResult := <-errorChan:
					failedJobName = runScriptResult.JobName
					returnedError = runScriptResult.Error
				case <-time.After(time.Second * 2):
					//If it went here , it will fail
				}

				Expect(failedJobName).To(Equal("fake-job-name"))
				Expect(returnedError.Error()).To(Equal("fake-command-error"))

				Expect(fs.FileExists("/fake-base-dir/fake-log-path/script-name.stdout.log")).To(BeTrue())
				Expect(fs.FileExists("/fake-base-dir/fake-log-path/script-name.stderr.log")).To(BeTrue())

				stdout, err := fs.ReadFileString("/fake-base-dir/fake-log-path/script-name.stdout.log")
				Expect(err).ToNot(HaveOccurred())
				Expect(stdout).To(Equal("fake-stdout"))

				stderr, err := fs.ReadFileString("/fake-base-dir/fake-log-path/script-name.stderr.log")
				Expect(err).ToNot(HaveOccurred())
				Expect(stderr).To(Equal("fake-stderr"))
			})
		})
	})
})
