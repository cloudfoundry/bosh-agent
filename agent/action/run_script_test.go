package action_test

import (
	"errors"
	"time"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	fakeapplyspec "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec/fakes"
	"github.com/cloudfoundry/bosh-agent/agent/scriptrunner"
	fakescript "github.com/cloudfoundry/bosh-agent/agent/scriptrunner/fakes"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("RunScript", func() {
	var (
		runScriptAction       action.RunScriptAction
		fakeJobScriptProvider *fakescript.FakeJobScriptProvider
		specService           *fakeapplyspec.FakeV1Service
		log                   logger.Logger
		options               map[string]interface{}
		scriptName            string
	)

	createFakeJob := func(jobName string) {
		spec := applyspec.JobTemplateSpec{Name: jobName}
		specService.Spec.JobSpec.JobTemplateSpecs = append(specService.Spec.JobSpec.JobTemplateSpecs, spec)
	}

	BeforeEach(func() {
		log = logger.NewLogger(logger.LevelNone)
		fakeJobScriptProvider = &fakescript.FakeJobScriptProvider{}
		specService = fakeapplyspec.NewFakeV1Service()
		createFakeJob("fake-job-1")
		runScriptAction = action.NewRunScript(fakeJobScriptProvider, specService, log)
		scriptName = "run-me"
		options = make(map[string]interface{})
	})

	It("is asynchronous", func() {
		Expect(runScriptAction.IsAsynchronous()).To(BeTrue())
	})

	It("is not persistent", func() {
		Expect(runScriptAction.IsPersistent()).To(BeFalse())
	})

	Context("when script exists", func() {
		var existingScript *fakescript.FakeScript

		BeforeEach(func() {
			existingScript = &fakescript.FakeScript{}
			existingScript.TagReturns("fake-job-1")
			existingScript.PathReturns("path/to/script1")
			existingScript.ExistsReturns(true)
			fakeJobScriptProvider.GetReturns(existingScript)
		})

		It("is executed", func() {
			existingScript.RunReturns(nil)

			results, err := runScriptAction.Run(scriptName, options)
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(Equal(map[string]string{"fake-job-1": "executed"}))
		})

		It("gives an error when script fails", func() {
			existingScript.RunReturns(errors.New("fake-error"))

			results, err := runScriptAction.Run(scriptName, options)
			Expect(err).To(HaveOccurred())
			Expect(results).To(Equal(map[string]string{"fake-job-1": "failed"}))
		})
	})

	Context("when running scripts concurrently", func() {
		var existingScript1 *fakescript.FakeScript
		var existingScript2 *fakescript.FakeScript

		BeforeEach(func() {
			existingScript1 = &fakescript.FakeScript{}
			existingScript1.TagReturns("fake-job-1")
			existingScript1.PathReturns("path/to/script1")
			existingScript1.ExistsReturns(true)

			createFakeJob("fake-job-2")
			existingScript2 = &fakescript.FakeScript{}
			existingScript2.TagReturns("fake-job-2")
			existingScript2.PathReturns("path/to/script2")
			existingScript2.ExistsReturns(true)

			fakeJobScriptProvider.GetStub = func(jobName string, relativePath string) scriptrunner.Script {
				if jobName == "fake-job-1" {
					return existingScript1
				} else if jobName == "fake-job-2" {
					return existingScript2
				}
				return nil
			}
		})

		It("is executed and both scripts pass", func() {
			existingScript1.RunReturns(nil)
			existingScript2.RunReturns(nil)

			results, err := runScriptAction.Run(scriptName, options)
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(Equal(map[string]string{"fake-job-1": "executed", "fake-job-2": "executed"}))
		})

		It("returns two failed statuses when both scripts fail", func() {
			existingScript1.RunReturns(errors.New("fake-error"))
			existingScript2.RunReturns(errors.New("fake-error"))

			results, err := runScriptAction.Run(scriptName, options)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("2 of 2 run-me scripts failed. Failed Jobs:"))
			Expect(err.Error()).Should(ContainSubstring("fake-job-1"))
			Expect(err.Error()).Should(ContainSubstring("fake-job-2"))
			Expect(err.Error()).ShouldNot(ContainSubstring("Successful Jobs"))

			Expect(results).To(Equal(map[string]string{"fake-job-1": "failed", "fake-job-2": "failed"}))
		})

		It("returns one failed status when first script fail and second script pass, and when one fails continue waiting for unfinished tasks", func() {
			existingScript1.RunStub = func() error {
				time.Sleep(2 * time.Second)
				return errors.New("fake-error")
			}
			existingScript2.RunReturns(nil)

			results, err := runScriptAction.Run(scriptName, options)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("1 of 2 run-me scripts failed. Failed Jobs: fake-job-1. Successful Jobs: fake-job-2."))
			Expect(results).To(Equal(map[string]string{"fake-job-1": "failed", "fake-job-2": "executed"}))
		})

		It("returns one failed status when first script pass and second script fail", func() {
			existingScript1.RunReturns(nil)
			existingScript2.RunReturns(errors.New("fake-error"))

			results, err := runScriptAction.Run(scriptName, options)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("1 of 2 run-me scripts failed. Failed Jobs: fake-job-2. Successful Jobs: fake-job-1."))
			Expect(results).To(Equal(map[string]string{"fake-job-1": "executed", "fake-job-2": "failed"}))
		})

		It("wait for scripts to finish", func() {
			existingScript1.RunStub = func() error {
				time.Sleep(2 * time.Second)
				return nil
			}
			existingScript2.RunReturns(nil)

			results, err := runScriptAction.Run(scriptName, options)
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(Equal(map[string]string{"fake-job-1": "executed", "fake-job-2": "executed"}))
		})
	})

	Context("when script does not exist", func() {
		var nonExistingScript *fakescript.FakeScript

		BeforeEach(func() {
			nonExistingScript = &fakescript.FakeScript{}
			nonExistingScript.ExistsReturns(false)
			fakeJobScriptProvider.GetReturns(nonExistingScript)
		})

		It("does not return a status for that script", func() {
			results, err := runScriptAction.Run(scriptName, options)
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(Equal(map[string]string{}))
		})
	})
})
