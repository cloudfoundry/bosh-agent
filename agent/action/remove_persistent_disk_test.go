package action

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	platformfakes "github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/v2/settings/fakes"
)

var _ = Describe("RemovePersistentDiskAction", func() {
	var (
		action          RemovePersistentDiskAction
		settingsService *fakesettings.FakeSettingsService
		platform        *platformfakes.FakePlatform
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		platform = &platformfakes.FakePlatform{}
		action = NewRemovePersistentDiskAction(settingsService, platform)
	})

	Context("when the disk has recorded settings", func() {
		BeforeEach(func() {
			settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{
				"diskCID": {ID: "diskCID", DeviceID: "6000c292-f726-52fa-eb33-8715920f9a92"},
			}
		})

		It("releases the device and updates persistent disk settings", func() {
			result, err := action.Run("diskCID")

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(map[string]string{}))

			Expect(platform.RemovePersistentDiskDeviceCallCount()).To(Equal(1))
			Expect(platform.RemovePersistentDiskDeviceArgsForCall(0).DeviceID).
				To(Equal("6000c292-f726-52fa-eb33-8715920f9a92"))

			Expect(settingsService.RemovePersistentDiskSettingsCallCount).To(Equal(1))
			Expect(settingsService.RemovePersistentDiskSettingsLastArg).To(Equal("diskCID"))
		})

		It("releases the device before dropping the settings", func() {
			// The settings carry the device ID, so dropping them first would leave
			// nothing to resolve the device with.
			settingsService.RemovePersistentDiskSettingsError = errors.New("boom")

			_, err := action.Run("diskCID")

			Expect(err).To(HaveOccurred())
			Expect(platform.RemovePersistentDiskDeviceCallCount()).To(Equal(1))
		})

		Context("when releasing the device fails", func() {
			BeforeEach(func() {
				platform.RemovePersistentDiskDeviceReturns(errors.New("could not release"))
			})

			It("returns the error and leaves the settings in place", func() {
				_, err := action.Run("diskCID")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Removing persistent disk device"))
				Expect(settingsService.RemovePersistentDiskSettingsCallCount).To(Equal(0))
			})
		})
	})

	Context("when the disk has no recorded settings", func() {
		// Reached when a detach is retried or rolled back; there is no device to
		// release, and that must not fail the action.
		It("skips the device removal and still updates settings", func() {
			result, err := action.Run("diskCID")

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(map[string]string{}))
			Expect(platform.RemovePersistentDiskDeviceCallCount()).To(Equal(0))
			Expect(settingsService.RemovePersistentDiskSettingsCallCount).To(Equal(1))
		})
	})

	Context("when reading settings fails", func() {
		BeforeEach(func() {
			settingsService.GetAllPersistentDiskSettingsError = errors.New("could not read")
		})

		It("should raise error", func() {
			_, err := action.Run("diskCID")
			Expect(err).To(HaveOccurred())
		})
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
