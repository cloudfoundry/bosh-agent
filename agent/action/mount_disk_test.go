package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("MountDiskAction", func() {
	var (
		settingsService *fakesettings.FakeSettingsService
		platform        *platformfakes.FakePlatform
		mountDiskAction action.MountDiskAction
		logger          boshlog.Logger
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		platform = &platformfakes.FakePlatform{}
		dirProvider := boshdirs.NewProvider("/fake-base-dir")
		logger = boshlog.NewLogger(boshlog.LevelNone)
		mountDiskAction = action.NewMountDisk(settingsService, platform, dirProvider, logger)
	})

	AssertActionIsAsynchronous(mountDiskAction)
	AssertActionIsNotPersistent(mountDiskAction)
	AssertActionIsLoggable(mountDiskAction)

	AssertActionIsNotResumable(mountDiskAction)
	AssertActionIsNotCancelable(mountDiskAction)

	Describe("Run", func() {
		Context("when settings can be loaded", func() {
			Context("when a disk hint is NOT passed in the mountDiskAction arguments", func() {
				Context("when disk cid can be resolved to a device path from infrastructure settings", func() {
					BeforeEach(func() {
						settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{
							"fake-disk-cid": {
								Path:     "fake-device-path",
								VolumeID: "fake-volume-id",
								ID:       "fake-disk-cid",
							},
						}
					})

					Context("when adjusting partitioning fails", func() {
						BeforeEach(func() {
							platform.AdjustPersistentDiskPartitioningReturns(errors.New("fake-adjust-persistent-disk-partitioning-err"))
						})

						It("returns error after trying to adjust partitioning", func() {
							_, err := mountDiskAction.Run("fake-disk-cid")
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("fake-adjust-persistent-disk-partitioning-err"))
							Expect(platform.AdjustPersistentDiskPartitioningCallCount()).To(Equal(1))
							Expect(platform.MountPersistentDiskCallCount()).To(Equal(0))
							Expect(settingsService.SavePersistentDiskSettingsCallCount).To(Equal(0))

							diskSettings, mntPt := platform.AdjustPersistentDiskPartitioningArgsForCall(0)
							Expect(diskSettings).To(Equal(boshsettings.DiskSettings{
								ID:       "fake-disk-cid",
								VolumeID: "fake-volume-id",
								Path:     "fake-device-path",
							}))
							Expect(mntPt).To(boshassert.MatchPath("/fake-base-dir/store"))
						})
					})

					Context("when mounting succeeds", func() {
						It("returns without an error after mounting store directory", func() {
							result, err := mountDiskAction.Run("fake-disk-cid")
							Expect(err).NotTo(HaveOccurred())
							Expect(result).To(Equal(map[string]string{}))

							Expect(platform.AdjustPersistentDiskPartitioningCallCount()).To(Equal(1))
							Expect(platform.MountPersistentDiskCallCount()).To(Equal(1))

							diskSettings, mntPt := platform.MountPersistentDiskArgsForCall(0)
							Expect(diskSettings).To(Equal(boshsettings.DiskSettings{
								ID:       "fake-disk-cid",
								VolumeID: "fake-volume-id",
								Path:     "fake-device-path",
							}))
							Expect(mntPt).To(boshassert.MatchPath("/fake-base-dir/store"))
						})

						It("does not save disk hint", func() {
							result, err := mountDiskAction.Run("fake-disk-cid")
							Expect(err).NotTo(HaveOccurred())
							Expect(result).To(Equal(map[string]string{}))
							Expect(settingsService.SavePersistentDiskSettingsCallCount).To(Equal(0))
						})
					})

					Context("when mounting fails", func() {
						BeforeEach(func() {
							platform.MountPersistentDiskReturns(errors.New("fake-mount-persistent-disk-err"))
						})

						It("returns error after trying to mount store directory", func() {
							_, err := mountDiskAction.Run("fake-disk-cid")
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("fake-mount-persistent-disk-err"))
							Expect(settingsService.SavePersistentDiskSettingsCallCount).To(Equal(0))
						})
					})
				})

				Context("when disk cid cannot be resolved to a device path from infrastructure settings", func() {
					BeforeEach(func() {
						settingsService.GetPersistentDiskSettingsError = errors.New("Persistent disk with volume id 'fake-unknown-disk-cid' could not be found")
					})

					It("returns error", func() {
						_, err := mountDiskAction.Run("fake-unknown-disk-cid")
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("Reading persistent disk settings: Persistent disk with volume id 'fake-unknown-disk-cid' could not be found"))
					})
				})
			})
		})

		Context("when settings cannot be loaded", func() {
			It("returns error", func() {
				settingsService.LoadSettingsError = errors.New("fake-load-settings-err")

				_, err := mountDiskAction.Run("fake-disk-cid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-load-settings-err"))
			})
		})
	})
})
