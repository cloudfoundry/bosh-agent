package action_test

import (
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"

	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	. "github.com/cloudfoundry/bosh-agent/agent/action"
)

var _ = Describe("UnmountDiskAction", func() {
	var (
		platform             *fakeplatform.FakePlatform
		action               UnmountDiskAction
		persistentDiskHints  map[string]boshsettings.DiskSettings
		expectedDiskSettings boshsettings.DiskSettings
		settingsService      *fakesettings.FakeSettingsService
	)

	BeforeEach(func() {
		platform = fakeplatform.NewFakePlatform()

		settingsService = &fakesettings.FakeSettingsService{
			Settings: boshsettings.Settings{
				Disks: boshsettings.Disks{
					Persistent: map[string]interface{}{
						"vol-123": map[string]interface{}{
							"volume_id":      "2",
							"path":           "/dev/sdf",
							"lun":            "0",
							"host_device_id": "fake-host-device-id",
						},
					},
				},
				Env: boshsettings.Env{
					PersistentDiskFS: "ext4",
				},
			},
		}
		action = NewUnmountDisk(settingsService, platform)

		persistentDiskHints = map[string]boshsettings.DiskSettings{
			"1": {ID: "1", Path: "abc"},
			"2": {ID: "2", Path: "def"},
			"3": {ID: "3", Path: "ghi"},
		}
		settingsService.PersistentDiskHints = persistentDiskHints

		expectedDiskSettings = boshsettings.DiskSettings{
			ID:             "vol-123",
			VolumeID:       "2",
			Path:           "/dev/sdf",
			FileSystemType: "ext4",
			Lun:            "0",
			HostDeviceID:   "fake-host-device-id",
		}
	})

	AssertActionIsAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Context("check for disk in registry", func() {

		Context("disk found in registry", func() {
			Context("disk is mounted", func() {
				It("unmounts disk successfully", func() {
					platform.UnmountPersistentDiskDidUnmount = true

					result, err := action.Run("vol-123")
					Expect(err).ToNot(HaveOccurred())
					boshassert.MatchesJSONString(GinkgoT(), result, `{"message":"Unmounted partition of {ID:vol-123 DeviceID: VolumeID:2 Lun:0 HostDeviceID:fake-host-device-id Path:/dev/sdf FileSystemType:ext4 MountOptions:[]}"}`)

					Expect(platform.UnmountPersistentDiskSettings).To(Equal(expectedDiskSettings))
				})
			})
			Context("disk is not mounted", func() {
				It("returns message and no error", func() {
					platform.UnmountPersistentDiskDidUnmount = false

					result, err := action.Run("vol-123")
					Expect(err).ToNot(HaveOccurred())
					boshassert.MatchesJSONString(GinkgoT(), result, `{"message":"Partition of {ID:vol-123 DeviceID: VolumeID:2 Lun:0 HostDeviceID:fake-host-device-id Path:/dev/sdf FileSystemType:ext4 MountOptions:[]} is not mounted"}`)

					Expect(platform.UnmountPersistentDiskSettings).To(Equal(expectedDiskSettings))
				})
			})
		})

		Context("disk is not found in the registry", func() {
			It("returns error", func() {
				_, err := action.Run("vol-456")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Persistent disk with volume id '%s' could not be found", "vol-456")))
			})
		})
	})

	Context("check for disk in persistent hints file", func() {
		BeforeEach(func() {
			settingsService.PersistentDiskHintWasFound = true
		})

		Context("disk not found in persistent hints file", func() {
			It("returns error", func() {
				settingsService.PersistentDiskHintWasFound = false

				_, err := action.Run("1")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Persistent disk with volume id '%s' could not be found", "1")))
			})
		})

		Context("failed to retrieve disk hint by disk ID", func() {
			It("propagates error", func() {
				settingsService.GetPersistentDiskHintError = errors.New("test error")

				_, err := action.Run("1")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("test error"))
			})
		})

		Context("platform unmount operation succeeds", func() {
			BeforeEach(func() {
				platform.UnmountPersistentDiskDidUnmount = true
				settingsService.GetPersistentDiskHintResult = boshsettings.DiskSettings{
					ID:   "1",
					Path: "abc",
				}
			})

			It("removes hints entry from hints file", func() {

				result, err := action.Run("1")
				Expect(err).ToNot(HaveOccurred())
				boshassert.MatchesJSONString(GinkgoT(), result, `{"message":"Unmounted partition of {ID:1 DeviceID: VolumeID: Lun: HostDeviceID: Path:abc FileSystemType: MountOptions:[]}"}`)

				Expect(platform.UnmountPersistentDiskSettings).To(Equal(persistentDiskHints["1"]))
				Expect(settingsService.RemovePersistentDiskHintsCallCount).To(Equal(1))
			})

			It("wraps error when failed to remove hint entry", func() {
				settingsService.RemovePersistentDiskHintsError = errors.New("file access issue")

				_, err := action.Run("1")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Could not delete disk hint for disk ID %s. Error: %s", "1", "file access issue")))
			})
		})

		Context("platform unmount operation fails", func() {
			It("propagates error", func() {
				platform.UnmountPersistentDiskErr = errors.New("unmount platform error")

				_, err := action.Run("1")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unmounting persistent disk"))
				Expect(err.Error()).To(ContainSubstring("unmount platform error"))
			})
		})

		Context("platform unmount does not return an error", func() {
			BeforeEach(func() {
				platform.UnmountPersistentDiskDidUnmount = false
				settingsService.GetPersistentDiskHintResult = boshsettings.DiskSettings{
					ID:   "1",
					Path: "abc",
				}
			})

			It("action returns message and no error", func() {
				result, err := action.Run("1")
				Expect(err).ToNot(HaveOccurred())
				boshassert.MatchesJSONString(GinkgoT(), result, `{"message":"Partition of {ID:1 DeviceID: VolumeID: Lun: HostDeviceID: Path:abc FileSystemType: MountOptions:[]} is not mounted"}`)

			})

			It("does not try to remove hint entry from hints file", func() {
				action.Run("1")
				Expect(platform.UnmountPersistentDiskSettings).To(Equal(persistentDiskHints["1"]))
				Expect(settingsService.RemovePersistentDiskHintsCallCount).To(Equal(0))
			})
		})

		Context("platform unmount did not unmount disk", func() {

			It("returns message", func() {

			})

			It("returns message and does not try to remove hint entry from hints file", func() {
				platform.UnmountPersistentDiskDidUnmount = false
				settingsService.GetPersistentDiskHintResult = boshsettings.DiskSettings{
					ID:   "1",
					Path: "abc",
				}

			})
		})

		Context("unmount failed", func() {
			It("propagates error", func() {

			})
		})

	})
})
