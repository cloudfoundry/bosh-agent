package action_test

import (
	"errors"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	fakeapplyspec "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec/fakes"
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
		specService.Spec.JobSpec.JobTemplateSpecs = append(specService.Spec.JobSpec.JobTemplateSpecs, applyspec.JobTemplateSpec{Name: jobName})
	}

	BeforeEach(func() {
		log = logger.NewLogger(logger.LevelNone)
		fakeJobScriptProvider = &fakescript.FakeJobScriptProvider{}
		specService = fakeapplyspec.NewFakeV1Service()
		createFakeJob("fake_job")
		runScriptAction = action.NewRunScript(fakeJobScriptProvider, specService, log)
		scriptName = "run-me"
		options = make(map[string]interface{})
	})

	It("is synchronous", func() {
		Expect(runScriptAction.IsAsynchronous()).To(BeTrue())
	})

	It("is not persistent", func() {
		Expect(runScriptAction.IsPersistent()).To(BeFalse())
	})

	Context("when script exists", func() {

		var existingScript *fakescript.FakeScript

		BeforeEach(func() {
			existingScript = &fakescript.FakeScript{}
			existingScript.ExistsReturns(true)
			fakeJobScriptProvider.GetReturns(existingScript)
		})

		It("is executed", func() {
			results, err := runScriptAction.Run(scriptName, options)

			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(Equal(map[string]string{"fake_job": "executed"}))
		})

		It("gives an error when script fails", func() {
			existingScript.RunReturns("stdout from before the error", "stderr from before the error", errors.New("fake-generic-run-script-error"))

			results, err := runScriptAction.Run(scriptName, options)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("1 of 1 run-me scripts failed. See logs for details."))
			Expect(results).To(Equal(map[string]string{"fake_job": "failed"}))
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
