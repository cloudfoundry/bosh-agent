package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"

	"path/filepath"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/platform/cert/certfakes"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	"github.com/cloudfoundry/bosh-utils/logger"

	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("UpdateSettings", func() {
	var (
		action            UpdateSettingsAction
		certManager       *certfakes.FakeManager
		settingsService   *fakesettings.FakeSettingsService
		log               logger.Logger
		platform          *platformfakes.FakePlatform
		newUpdateSettings boshsettings.UpdateSettings
		fileSystem        *fakesys.FakeFileSystem
	)

	BeforeEach(func() {
		log = logger.NewLogger(logger.LevelNone)
		certManager = new(certfakes.FakeManager)
		settingsService = &fakesettings.FakeSettingsService{}

		platform = &platformfakes.FakePlatform{}
		fileSystem = fakesys.NewFakeFileSystem()
		platform.GetFsReturns(fileSystem)

		action = NewUpdateSettings(settingsService, platform, certManager, log)
		newUpdateSettings = boshsettings.UpdateSettings{}
	})

	AssertActionIsAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Context("on success", func() {
		It("returns 'updated'", func() {
			result, err := action.Run(newUpdateSettings)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("updated"))
		})

		It("writes the updated settings to a file", func() {
			action.Run(newUpdateSettings)
			expectedPath := filepath.Join(platform.GetDirProvider().BoshDir(), "update_settings.json")
			exists := platform.GetFs().FileExists(expectedPath)
			Expect(exists).To(Equal(true))
		})
	})

	Context("when it cannot write the update settings file", func() {
		BeforeEach(func() {
			fileSystem.WriteFileError = errors.New("Fake write error")
		})

		It("returns an error", func() {
			_, err := action.Run(newUpdateSettings)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Fake write error"))
		})
	})

	Context("when updating the certificates fails", func() {
		BeforeEach(func() {
			log = logger.NewLogger(logger.LevelNone)
			certManager = new(certfakes.FakeManager)
			certManager.UpdateCertificatesReturns(errors.New("Error"))
			action = NewUpdateSettings(settingsService, platform, certManager, log)
		})

		It("returns the error", func() {
			result, err := action.Run(newUpdateSettings)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
	})

	It("loads settings", func() {
		_, err := action.Run(newUpdateSettings)
		Expect(err).ToNot(HaveOccurred())
		Expect(settingsService.SettingsWereLoaded).To(BeTrue())
	})

	Context("when loading the settings fails", func() {
		It("returns an error", func() {
			settingsService.LoadSettingsError = errors.New("nope")
			_, err := action.Run(newUpdateSettings)
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
			settingsService.GetPersistentDiskSettingsError = errors.New("Disk DNE")
		})

		It("returns the error", func() {
			_, err := action.Run(newUpdateSettings)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Fetching disk settings: Disk DNE"))
		})
	})

	It("associates the disks", func() {
		settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{
			"fake-disk-id": {
				VolumeID:     "fake-disk-volume-id",
				ID:           "fake-disk-id",
				DeviceID:     "fake-disk-device-id",
				Path:         "fake-disk-path",
				Lun:          "fake-disk-lun",
				HostDeviceID: "fake-disk-host-device-id",
			},
			"fake-disk-id-2": {
				VolumeID:     "fake-disk-volume-id-2",
				ID:           "fake-disk-id-2",
				DeviceID:     "fake-disk-device-id-2",
				Path:         "fake-disk-path-2",
				Lun:          "fake-disk-lun-2",
				HostDeviceID: "fake-disk-host-device-id-2",
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

		result, err := action.Run(boshsettings.UpdateSettings{
			DiskAssociations: []boshsettings.DiskAssociation{
				diskAssociation,
				diskAssociation2,
			},
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("updated"))
		Expect(platform.AssociateDiskCallCount()).To(Equal(2))

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
