package action_test

import (
	"errors"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/cloudfoundry/bosh-agent/agent/scriptrunner/fakes"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"
)

var _ = Describe("RunScript", func() {
	var (
		fakeScriptProvider *FakeScriptProvider
		action             RunScriptAction
		logger             Logger
		options            map[string]interface{}
		scriptPaths        []string
	)

	BeforeEach(func() {
		logger = NewLogger(LevelNone)
		fakeScriptProvider = NewFakeScriptProvider()
		action = NewRunScript(fakeScriptProvider, logger)
		scriptPaths = []string{"run-me"}
		options = make(map[string]interface{})
	})

	It("is synchronous", func() {
		Expect(action.IsAsynchronous()).To(BeTrue())
	})

	It("is not persistent", func() {
		Expect(action.IsPersistent()).To(BeFalse())
	})

	Context("when script exists", func() {
		It("is executed", func() {
			fakeScriptProvider.Script.ExistsBool = true
			preStart, err := action.Run(scriptPaths, options)

			Expect(err).ToNot(HaveOccurred())
			Expect(preStart).To(Equal("executed"))
		})

		It("gives an error when script fails", func() {
			fakeScriptProvider.Script.ExistsBool = true
			fakeScriptProvider.Script.RunError = errors.New("fake-generic-run-script-error")
			preStart, err := action.Run(scriptPaths, options)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-generic-run-script-error"))
			Expect(preStart).To(Equal("failed"))
		})
	})

	Context("when script does not exist", func() {
		It("does not give an error", func() {
			fakeScriptProvider.Script.ExistsBool = false
			preStart, err := action.Run(scriptPaths, options)

			Expect(err).ToNot(HaveOccurred())
			Expect(preStart).To(Equal("missing"))
		})
	})
})
