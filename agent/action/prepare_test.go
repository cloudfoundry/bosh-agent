package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	fakeappl "github.com/cloudfoundry/bosh-agent/agent/applier/fakes"
)

var _ = Describe("PrepareAction", func() {
	var (
		applier       *fakeappl.FakeApplier
		prepareAction action.PrepareAction
	)

	BeforeEach(func() {
		applier = fakeappl.NewFakeApplier()
		prepareAction = action.NewPrepare(applier)
	})

	AssertActionIsAsynchronous(prepareAction)
	AssertActionIsNotPersistent(prepareAction)
	AssertActionIsLoggable(prepareAction)

	AssertActionIsNotResumable(prepareAction)
	AssertActionIsNotCancelable(prepareAction)

	Describe("Run", func() {
		desiredApplySpec := boshas.V1ApplySpec{ConfigurationHash: "fake-desired-config-hash"}

		It("runs applier to prepare vm for future configuration with desired apply spec", func() {
			_, err := prepareAction.Run(desiredApplySpec)
			Expect(err).ToNot(HaveOccurred())
			Expect(applier.Prepared).To(BeTrue())
			Expect(applier.PrepareDesiredApplySpec).To(Equal(desiredApplySpec))
		})

		Context("when applier succeeds preparing vm", func() {
			It("returns 'applied' after setting desired spec as current spec", func() {
				value, err := prepareAction.Run(desiredApplySpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal("prepared"))
			})
		})

		Context("when applier fails preparing vm", func() {
			It("returns error", func() {
				applier.PrepareError = errors.New("fake-prepare-error")

				_, err := prepareAction.Run(desiredApplySpec)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-prepare-error"))
			})
		})
	})
})
