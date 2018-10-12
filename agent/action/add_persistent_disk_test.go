package action

import (
	"github.com/cloudfoundry/bosh-agent/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("AddPersistentDiskAction", func() {
	var (
		action          AddPersistentDiskAction
		settingsService *fakesettings.FakeSettingsService
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		action = NewAddPersistentDiskAction(settingsService)
	})

	It("updates persistent disk settings", func() {
		result, err := action.Run("diskCID", "/dev/sdb")

		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(map[string]string{}))
		Expect(settingsService.SavePersistentDiskSettingsCallCount).To(Equal(1))
		Expect(settingsService.SavePersistentDiskSettingsLastArg).To(Equal(settings.DiskSettings{
			ID:       "diskCID",
			Path:     "/dev/sdb",
			VolumeID: "/dev/sdb",
		}))
	})

	Context("when saving settings fails", func() {
		BeforeEach(func() {
			settingsService.SavePersistentDiskSettingsErr = errors.New("Could not save")
		})

		It("should raise error", func() {
			_, err := action.Run("diskCID", "/dev/sdb")
			Expect(err).To(HaveOccurred())
		})
	})
})
