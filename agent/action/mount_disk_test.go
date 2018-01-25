package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("MountDiskAction", func() {
	var (
		settingsService *fakesettings.FakeSettingsService
		platform        *fakeplatform.FakePlatform
		action          MountDiskAction
		logger          boshlog.Logger
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		platform = fakeplatform.NewFakePlatform()
		dirProvider := boshdirs.NewProvider("/fake-base-dir")
		logger = boshlog.NewLogger(boshlog.LevelNone)
		action = NewMountDisk(settingsService, platform, dirProvider, logger)
	})

	AssertActionIsAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Describe("Run", func() {
		Context("when settings can be loaded", func() {
			Context("when a disk hint is NOT passed in the action arguments", func() {
				Context("when disk cid can be resolved to a device path from infrastructure settings", func() {
					BeforeEach(func() {
						settingsService.Settings.Disks.Persistent = map[string]interface{}{
							"fake-disk-cid": map[string]interface{}{
								"path":      "fake-device-path",
								"volume_id": "fake-volume-id",
							},
						}
					})

					Context("when mounting succeeds", func() {
						It("returns without an error after mounting store directory", func() {
							result, err := action.Run("fake-disk-cid")
							Expect(err).NotTo(HaveOccurred())
							Expect(result).To(Equal(map[string]string{}))

							Expect(platform.MountPersistentDiskSettings).To(Equal(boshsettings.DiskSettings{
								ID:       "fake-disk-cid",
								VolumeID: "fake-volume-id",
								Path:     "fake-device-path",
							}))
							Expect(platform.MountPersistentDiskMountPoint).To(boshassert.MatchPath("/fake-base-dir/store"))
						})

						It("does not save disk hint", func() {
							result, err := action.Run("fake-disk-cid")
							Expect(err).NotTo(HaveOccurred())
							Expect(result).To(Equal(map[string]string{}))
							Expect(settingsService.SavePersistentDiskHintCallCount).To(Equal(0))
						})
					})

					Context("when mounting fails", func() {
						It("returns error after trying to mount store directory", func() {
							platform.MountPersistentDiskErr = errors.New("fake-mount-persistent-disk-err")

							_, err := action.Run("fake-disk-cid")
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("fake-mount-persistent-disk-err"))
							Expect(settingsService.SavePersistentDiskHintCallCount).To(Equal(0))
						})
					})
				})

				Context("when disk cid cannot be resolved to a device path from infrastructure settings", func() {
					BeforeEach(func() {
						settingsService.Settings.Disks.Persistent = map[string]interface{}{
							"fake-known-disk-cid": "/dev/sdf",
						}
					})

					It("returns error", func() {
						_, err := action.Run("fake-unknown-disk-cid")
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("Persistent disk with volume id 'fake-unknown-disk-cid' could not be found"))
					})
				})
			})

			Context("when a disk hint is passed in the action arguments (we still need the disk settings to get the disk options from ENV)", func() {
				var diskHint interface{}

				BeforeEach(func() {
					settingsService.Settings.Disks.Persistent = map[string]interface{}{
						"fake-disk-cid": map[string]interface{}{
							"path":      "non-used-fake-device-path",
							"volume_id": "non-used-fake-volume-id",
						},
					}
				})

				Context("when the disk hint is a string", func() {
					BeforeEach(func() {
						diskHint = "disk_hint_string"
					})

					It("it should work as expected", func() {
						_, err := action.Run("hint-fake-disk-cid", diskHint)
						Expect(err).ToNot(HaveOccurred())
						Expect(platform.MountPersistentDiskSettings).To(Equal(boshsettings.DiskSettings{
							ID:       "hint-fake-disk-cid",
							VolumeID: "disk_hint_string",
							Path:     "disk_hint_string",
						}))
					})

					It("saves disk hint if mount succeeds", func() {
						_, err := action.Run("hint-fake-disk-cid", diskHint)
						Expect(err).ToNot(HaveOccurred())
						Expect(settingsService.SavePersistentDiskHintCallCount).To(Equal(1))
					})

					It("saves disk hint if mount fails", func() {
						platform.MountPersistentDiskErr = errors.New("fake-mount-persistent-disk-err")
						_, err := action.Run("hint-fake-disk-cid", diskHint)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-mount-persistent-disk-err"))
						Expect(settingsService.SavePersistentDiskHintCallCount).To(Equal(1))
					})
				})

				Context("when the hint is a map", func() {
					BeforeEach(func() {
						diskHint = map[string]interface{}{
							"volume_id":      "hint-fake-disk-volume-id",
							"id":             "hint-fake-disk-device-id",
							"path":           "hint-fake-disk-path",
							"lun":            "hint-fake-disk-lun",
							"host_device_id": "hint-fake-disk-host-device-id",
						}
					})

					It("it should work as expected", func() {
						_, err := action.Run("hint-fake-disk-cid", diskHint)
						Expect(err).ToNot(HaveOccurred())
						Expect(platform.MountPersistentDiskSettings).To(Equal(boshsettings.DiskSettings{
							ID:           "hint-fake-disk-cid",
							DeviceID:     "hint-fake-disk-device-id",
							VolumeID:     "hint-fake-disk-volume-id",
							Path:         "hint-fake-disk-path",
							Lun:          "hint-fake-disk-lun",
							HostDeviceID: "hint-fake-disk-host-device-id",
						}))
					})

					It("saves disk hint if mount succeeds", func() {
						_, err := action.Run("hint-fake-disk-cid", diskHint)
						Expect(err).ToNot(HaveOccurred())
						Expect(settingsService.SavePersistentDiskHintCallCount).To(Equal(1))
					})

					It("saves disk hint if mount fails", func() {
						platform.MountPersistentDiskErr = errors.New("fake-mount-persistent-disk-err")
						_, err := action.Run("hint-fake-disk-cid", diskHint)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-mount-persistent-disk-err"))
						Expect(settingsService.SavePersistentDiskHintCallCount).To(Equal(1))
					})
				})
			})
		})

		Context("when settings cannot be loaded", func() {
			It("returns error", func() {
				settingsService.LoadSettingsError = errors.New("fake-load-settings-err")

				_, err := action.Run("fake-disk-cid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-load-settings-err"))
			})
		})
	})
})
