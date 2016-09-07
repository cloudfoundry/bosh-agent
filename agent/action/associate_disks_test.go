package action_test

import (
	"errors"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakelog "github.com/cloudfoundry/bosh-agent/logger/fakes"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
)

var _ = Describe("Associate Disks Action", func() {

	var (
		action          AssociateDisksAction
		logger          *fakelog.FakeLogger
		settingsService *fakesettings.FakeSettingsService
		platform        *fakeplatform.FakePlatform
		diskAssociation DiskAssociation
	)

	BeforeEach(func() {
		logger = &fakelog.FakeLogger{}
		settingsService = &fakesettings.FakeSettingsService{}
		platform = &fakeplatform.FakePlatform{}
		action = NewAssociateDisks(settingsService, platform, logger)
	})

	It("loads settings", func() {
		_, err := action.Run(DiskAssociations{})
		Expect(err).ToNot(HaveOccurred())
		Expect(settingsService.SettingsWereLoaded).To(BeTrue())
	})

	It("returns a success string", func() {
		result, err := action.Run(DiskAssociations{})
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("associated"))
	})

	Context("when reloading the settins fails", func() {
		It("returns an error", func() {
			settingsService.LoadSettingsError = errors.New("nope")
			_, err := action.Run(DiskAssociations{})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the settings does not contain the disk", func() {
		BeforeEach(func() {
			diskAssociation = DiskAssociation{
				Name:    "fake-disk-name",
				DiskCID: "fake-disk-id",
			}
			settingsService.Settings = boshsettings.Settings{}
		})

		It("returns the error", func() {
			_, err := action.Run(DiskAssociations{
				Associations: []DiskAssociation{
					diskAssociation,
				},
			})

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

		diskAssociation := DiskAssociation{
			Name:    "fake-disk-name",
			DiskCID: "fake-disk-id",
		}

		diskAssociation2 := DiskAssociation{
			Name:    "fake-disk-name2",
			DiskCID: "fake-disk-id-2",
		}

		result, err := action.Run(DiskAssociations{
			Associations: []DiskAssociation{
				diskAssociation,
				diskAssociation2,
			},
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("associated"))

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

	Context("when associating a disk fails", func() {
		It("returns an error", func() {
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
					},
				},
			}

			diskAssociation := DiskAssociation{
				Name:    "fake-disk-name",
				DiskCID: "fake-disk-id",
			}

			platform.AssociateDiskError = errors.New("not today")

			_, err := action.Run(DiskAssociations{
				Associations: []DiskAssociation{
					diskAssociation,
				},
			})

			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(platform.AssociateDiskError))
		})
	})
})
