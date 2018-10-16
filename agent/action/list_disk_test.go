package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	bosherrors "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("ListDisk", func() {
	var (
		settingsService *fakesettings.FakeSettingsService
		platform        *platformfakes.FakePlatform
		logger          boshlog.Logger
		action          ListDiskAction
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		platform = &platformfakes.FakePlatform{}
		logger = boshlog.NewLogger(boshlog.LevelNone)
		action = NewListDisk(settingsService, platform, logger)

		platform.IsPersistentDiskMountedStub = func(settings boshsettings.DiskSettings) (bool, error) {
			if settings.Path == "/dev/sdb" || settings.Path == "/dev/sdc" {
				return true, nil
			}

			return false, nil
		}
	})

	AssertActionIsSynchronousForVersion(action, 1)
	AssertActionIsSynchronousForVersion(action, 2)
	AssertActionIsAsynchronousForVersion(action, 3)

	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Context("list disk run", func() {
		Context("persistent disks defined", func() {
			BeforeEach(func() {
				platform.IsPersistentDiskMountedStub = func(diskSettings boshsettings.DiskSettings) (bool, error) {
					expectedHints := []string{"/dev/sdb", "/dev/sdc", "abc", "def"}
					for _, mountedPath := range expectedHints {
						if mountedPath == diskSettings.Path {
							return true, nil
						}
					}
					return false, nil

				}

				settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{
					"1": {ID: "1", Path: "/dev/sdb"},
					"2": {ID: "2", Path: "/dev/sdc"},
					"3": {ID: "3", Path: "abc"},
					"4": {ID: "4", Path: "def"},
					"5": {ID: "5", Path: "xyz"},
				}
			})

			It("returns list of mounted disks", func() {
				value, err := action.Run()
				Expect(err).ToNot(HaveOccurred())
				values, ok := value.([]string)
				Expect(ok).To(BeTrue())
				Expect(values).To(ContainElement("1"))
				Expect(values).To(ContainElement("2"))
				Expect(values).To(ContainElement("3"))
				Expect(values).To(ContainElement("4"))
				Expect(values).ToNot(ContainElement("5"))
			})
		})

		Context("there are no disks", func() {
			It("returns an empty array", func() {
				result, err := action.Run()
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result).To(Equal(make([]string, 0)))
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
			Context("when persistent disk settings file does not exist", func() {
				It("should not return an error", func() {
					_, err := action.Run()
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when getting disk hint settings fails", func() {
				It("should return an error", func() {
					settingsService.GetAllPersistentDiskSettingsError = bosherrors.Error("fake get persistent disk hints error")

					_, err := action.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Getting persistent disk settings: fake get persistent disk hints error"))
				})
			})
		})
	})
})
