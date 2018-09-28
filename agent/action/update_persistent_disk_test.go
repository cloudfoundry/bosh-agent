package action

import (
	"github.com/cloudfoundry/bosh-agent/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("UpdatePersistentDiskAction", func() {
	var (
		action          UpdatePersistentDiskAction
		settingsService *fakesettings.FakeSettingsService
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		action = NewUpdatePersistentDiskAction(settingsService)
	})

	It("updates persistent disk settings", func() {
		result, err := action.Run("diskCID", "/dev/sdb")

		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("updated_persistent_disk"))
		Expect(settingsService.SavePersistentDiskHintCallCount).To(Equal(1))
		Expect(settingsService.SavePersistentDiskHintLastArg).To(Equal(settings.DiskSettings{
			ID:       "diskCID",
			Path:     "/dev/sdb",
			VolumeID: "/dev/sdb",
		}))
	})

	Context("when saving settings fails", func() {
		BeforeEach(func() {
			settingsService.SavePersistentDiskHintErr = errors.New("Could not save")
		})

		It("should raise error", func() {
			_, err := action.Run("diskCID", "/dev/sdb")
			Expect(err).To(HaveOccurred())
		})
	})
})
