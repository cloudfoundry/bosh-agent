package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/platform/cert/fakes"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	"github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("UpdateSettings", func() {
	var (
		updateAction      action.UpdateSettingsAction
		certManager       *fakes.FakeManager
		settingsService   *fakesettings.FakeSettingsService
		log               logger.Logger
		platform          *fakeplatform.FakePlatform
		newUpdateSettings boshsettings.UpdateSettings
	)

	BeforeEach(func() {
		log = logger.NewLogger(logger.LevelNone)
		certManager = new(fakes.FakeManager)
		settingsService = &fakesettings.FakeSettingsService{}
		platform = &fakeplatform.FakePlatform{}
		updateAction = action.NewUpdateSettings(settingsService, platform, certManager, log)
		newUpdateSettings = boshsettings.UpdateSettings{}
	})

	It("is asynchronous", func() {
		Expect(updateAction.IsAsynchronous()).To(BeTrue())
	})

	It("is not persistent", func() {
		Expect(updateAction.IsPersistent()).To(BeFalse())
	})

	It("returns 'updated' on success", func() {
		result, err := updateAction.Run(newUpdateSettings)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("updated"))
	})

	Context("when updating the certificates fails", func() {
		BeforeEach(func() {
			log = logger.NewLogger(logger.LevelNone)
			certManager = new(fakes.FakeManager)
			certManager.UpdateCertificatesReturns(errors.New("Error"))
			updateAction = action.NewUpdateSettings(settingsService, platform, certManager, log)
		})

		It("returns the error", func() {
			result, err := updateAction.Run(newUpdateSettings)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
	})

	It("loads settings", func() {
		_, err := updateAction.Run(newUpdateSettings)
		Expect(err).ToNot(HaveOccurred())
		Expect(settingsService.SettingsWereLoaded).To(BeTrue())
	})

	Context("when loading the settings fails", func() {
		It("returns an error", func() {
			settingsService.LoadSettingsError = errors.New("nope")
			_, err := updateAction.Run(newUpdateSettings)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the settings does not contain the disk", func() {
		var (
			diskAssociation   boshsettings.DiskAssociation
			newUpdateSettings boshsettings.UpdateSettings
		)

		BeforeEach(func() {
			diskAssociation = boshsettings.DiskAssociation{
				Name:    "fake-disk-name",
				DiskCID: "fake-disk-id",
			}
			newUpdateSettings = boshsettings.UpdateSettings{
				DiskAssociations: []boshsettings.DiskAssociation{diskAssociation},
			}
		})

		It("returns the error", func() {
			_, err := updateAction.Run(newUpdateSettings)

			Expect(err).To(HaveOccurred())
		})
	})

	It("associates the disks", func() {
		settingsService.Settings = boshsettings.Settings{
			Disks: boshsettings.Disks{
				Persistent: map[string]interface{}{
					"fake-disk-id": map[string]interface{}{
						"volume_id":      "fake-disk-volume-id",
						"id":             "fake-disk-device-id",
						"path":           "fake-disk-path",
						"lun":            "fake-disk-lun",
						"host_device_id": "fake-disk-host-device-id",
					},
					"fake-disk-id-2": map[string]interface{}{
						"volume_id":      "fake-disk-volume-id-2",
						"id":             "fake-disk-device-id-2",
						"path":           "fake-disk-path-2",
						"lun":            "fake-disk-lun-2",
						"host_device_id": "fake-disk-host-device-id-2",
					},
				},
			},
		}

		diskAssociation := boshsettings.DiskAssociation{
			Name:    "fake-disk-name",
			DiskCID: "fake-disk-id",
		}

		diskAssociation2 := boshsettings.DiskAssociation{
			Name:    "fake-disk-name2",
			DiskCID: "fake-disk-id-2",
		}

		result, err := updateAction.Run(boshsettings.UpdateSettings{
			DiskAssociations: []boshsettings.DiskAssociation{
				diskAssociation,
				diskAssociation2,
			},
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("updated"))

		actualDiskName, actualDiskSettings := platform.AssociateDiskArgsForCall(0)
		Expect(actualDiskName).To(Equal(diskAssociation.Name))
		Expect(actualDiskSettings).To(Equal(boshsettings.DiskSettings{
			ID:           "fake-disk-id",
			DeviceID:     "fake-disk-device-id",
			VolumeID:     "fake-disk-volume-id",
			Lun:          "fake-disk-lun",
			HostDeviceID: "fake-disk-host-device-id",
			Path:         "fake-disk-path",
		}))

		Expect(platform.AssociateDiskCallCount).To(Equal(2))

		actualDiskName, actualDiskSettings = platform.AssociateDiskArgsForCall(1)
		Expect(actualDiskName).To(Equal(diskAssociation2.Name))
		Expect(actualDiskSettings).To(Equal(boshsettings.DiskSettings{
			ID:           "fake-disk-id-2",
			DeviceID:     "fake-disk-device-id-2",
			VolumeID:     "fake-disk-volume-id-2",
			Lun:          "fake-disk-lun-2",
			HostDeviceID: "fake-disk-host-device-id-2",
			Path:         "fake-disk-path-2",
		}))

	})
})
