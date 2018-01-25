package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	bosherrors "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("ListDisk", func() {
	var (
		settingsService *fakesettings.FakeSettingsService
		platform        *fakeplatform.FakePlatform
		logger          boshlog.Logger
		action          ListDiskAction
		fs              *fakesys.FakeFileSystem
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		platform = fakeplatform.NewFakePlatform()
		logger = boshlog.NewLogger(boshlog.LevelNone)
		action = NewListDisk(settingsService, platform, logger)
		fs = fakesys.NewFakeFileSystem()
	})

	AssertActionIsSynchronousForVersion(action, 1)
	AssertActionIsSynchronousForVersion(action, 2)
	AssertActionIsAsynchronousForVersion(action, 3)

	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Context("list disk run", func() {
		Context("persistent disks defined in registry only", func() {
			BeforeEach(func() {
				platform.MountedDevicePaths = []string{"/dev/sdb", "/dev/sdc"}

				settingsService.Settings.Disks = boshsettings.Disks{
					Persistent: map[string]interface{}{
						"volume-1": "/dev/sda",
						"volume-2": "/dev/sdb",
						"volume-3": "/dev/sdc",
					},
				}
			})

			Context("platform mount check returns an error", func() {
				It("returns error", func() {
					platform.IsPersistentDiskMountedErr = bosherrors.Error("test")
					_, err := action.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Checking whether device"))
					Expect(err.Error()).To(ContainSubstring("is mounted"))
				})
			})

			It("processes disks from registry", func() {
				value, err := action.Run()
				Expect(err).ToNot(HaveOccurred())
				values, ok := value.([]string)
				Expect(ok).To(BeTrue())
				Expect(values).To(ContainElement("volume-2"))
				Expect(values).To(ContainElement("volume-3"))
				Expect(len(values)).To(Equal(2))

				Expect(settingsService.SettingsWereLoaded).To(BeTrue())
			})
		})

		Context("persistent disks defined in disk hints only", func() {
			BeforeEach(func() {
				settingsService.PersistentDiskHints = map[string]boshsettings.DiskSettings{
					"1": {ID: "1", Path: "abc"},
					"2": {ID: "2", Path: "def"},
					"3": {ID: "3", Path: "ghi"},
				}
			})

			Context("platform mount check returns an error", func() {
				It("returns error", func() {
					platform.IsPersistentDiskMountedErr = bosherrors.Error("test")
					_, err := action.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Checking whether device"))
					Expect(err.Error()).To(ContainSubstring("is mounted"))
				})
			})

			It("processes disks from persistent disk hints", func() {
				platform.MountedDevicePaths = []string{"abc", "def", "ghi"}

				value, err := action.Run()
				Expect(err).ToNot(HaveOccurred())
				values, ok := value.([]string)
				Expect(ok).To(BeTrue())
				Expect(values).To(ContainElement("1"))
				Expect(values).To(ContainElement("2"))
				Expect(values).To(ContainElement("3"))
				Expect(len(values)).To(Equal(3))

				Expect(settingsService.SettingsWereLoaded).To(BeTrue())
			})
		})

		Context("persistent disks defined in both registry and disk hints", func() {
			BeforeEach(func() {
				platform.MountedDevicePaths = []string{"/dev/sdb", "/dev/sdc", "abc", "def"}

				settingsService.Settings.Disks = boshsettings.Disks{
					Persistent: map[string]interface{}{
						"volume-1": "/dev/sda",
						"volume-2": "/dev/sdb",
						"volume-3": "/dev/sdc",
					},
				}

				settingsService.PersistentDiskHints = map[string]boshsettings.DiskSettings{
					"1": {ID: "1", Path: "abc"},
					"2": {ID: "2", Path: "def"},
					"3": {ID: "3", Path: "ghi"},
				}
			})

			It("returns list of disks containing both registry and disk hint disks", func() {
				value, err := action.Run()
				Expect(err).ToNot(HaveOccurred())
				values, ok := value.([]string)
				Expect(ok).To(BeTrue())
				Expect(values).To(ContainElement("volume-2"))
				Expect(values).To(ContainElement("volume-3"))
				Expect(values).To(ContainElement("1"))
				Expect(values).To(ContainElement("2"))
				Expect(values).ToNot(ContainElement("volume-1"))
				Expect(values).ToNot(ContainElement("3"))
			})

			Context("When there are duplicate entries", func() {
				BeforeEach(func() {
					settingsService.PersistentDiskHints = map[string]boshsettings.DiskSettings{
						"1":        {ID: "1", Path: "abc"},
						"volume-2": {ID: "volume-2", Path: "def"},
						"3":        {ID: "3", Path: "ghi"},
					}
				})

				It("eliminates the duplicates in the output", func() {
					value, err := action.Run()
					Expect(err).ToNot(HaveOccurred())
					values, ok := value.([]string)
					Expect(ok).To(BeTrue())
					Expect(len(values)).To(Equal(3))
					Expect(values).To(ContainElement("volume-2"))
					Expect(values).To(ContainElement("volume-3"))
					Expect(values).To(ContainElement("1"))
					Expect(values).ToNot(ContainElement("volume-1"))
					Expect(values).ToNot(ContainElement("3"))
				})
			})
		})
	})

	Context("error loading disk paths", func() {
		Context("when unable to loadsettings", func() {
			It("should return an error", func() {
				settingsService.LoadSettingsError = bosherrors.Error("fake loadsettings error")

				_, err := action.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Refreshing the settings: fake loadsettings error"))
			})
		})

		Context("when unable to load persistent disk hints", func() {
			Context("when persistent disk hints file does not exist", func() {
				It("should not return an error", func() {
					_, err := action.Run()
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when getting disk hint settings fails", func() {
				It("should return an error", func() {
					settingsService.GetPersistentDiskHintsError = bosherrors.Error("fake get persistent disk hints error")

					_, err := action.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Refreshing the disk hint settings: fake get persistent disk hints error"))
				})
			})
		})
	})
})
