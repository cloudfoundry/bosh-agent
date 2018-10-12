package action

import (
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("RemovePersistentDiskAction", func() {
	var (
		action          RemovePersistentDiskAction
		settingsService *fakesettings.FakeSettingsService
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		action = NewRemovePersistentDiskAction(settingsService)
	})

	It("updates persistent disk settings", func() {
		result, err := action.Run("diskCID")

		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(map[string]string{}))
		Expect(settingsService.RemovePersistentDiskSettingsCallCount).To(Equal(1))
		Expect(settingsService.RemovePersistentDiskSettingsLastArg).To(Equal("diskCID"))
	})

	Context("when removing settings fails", func() {
		BeforeEach(func() {
			settingsService.RemovePersistentDiskSettingsError = errors.New("Could not remove")
		})

		It("should raise error", func() {
			_, err := action.Run("diskCID")
			Expect(err).To(HaveOccurred())
		})
	})
})
