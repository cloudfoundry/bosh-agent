package action_test

import (
	"errors"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	fakescript "github.com/cloudfoundry/bosh-agent/agent/scriptrunner/fakes"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("RunScript", func() {
	var (
		runScriptAction    action.RunScriptAction
		fakeScriptProvider *fakescript.FakeScriptProvider
		log                logger.Logger
		options            map[string]interface{}
		scriptPaths        []string
	)

	BeforeEach(func() {
		log = logger.NewLogger(logger.LevelNone)
		fakeScriptProvider = &fakescript.FakeScriptProvider{}
		runScriptAction = action.NewRunScript(fakeScriptProvider, log)
		scriptPaths = []string{"run-me"}
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
			fakeScriptProvider.GetReturns(existingScript)
		})

		It("is executed", func() {
			preStart, err := runScriptAction.Run(scriptPaths, options)

			Expect(err).ToNot(HaveOccurred())
			Expect(preStart).To(Equal("executed"))
		})

		It("gives an error when script fails", func() {
			existingScript.RunReturns("stdout from before the error", "stderr from before the error", errors.New("fake-generic-run-script-error"))
			preStart, err := runScriptAction.Run(scriptPaths, options)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-generic-run-script-error"))
			Expect(preStart).To(Equal("failed"))
		})
	})

	Context("when script does not exist", func() {

		var nonExistingScript *fakescript.FakeScript

		BeforeEach(func() {
			nonExistingScript = &fakescript.FakeScript{}
			nonExistingScript.ExistsReturns(false)
			fakeScriptProvider.GetReturns(nonExistingScript)
		})

		It("does not give an error", func() {
			preStart, err := runScriptAction.Run(scriptPaths, options)

			Expect(err).ToNot(HaveOccurred())
			Expect(preStart).To(Equal("missing"))
		})
	})
})
