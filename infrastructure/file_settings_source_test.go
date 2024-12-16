package infrastructure_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/infrastructure"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("FileSettingsSource", func() {
	var (
		fs     *fakesys.FakeFileSystem
		source *infrastructure.FileSettingsSource
		logger boshlog.Logger
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		logger = boshlog.NewLogger(boshlog.LevelNone)
	})

	Describe("PublicSSHKeyForUsername", func() {
		It("returns an empty string", func() {
			publicKey, err := source.PublicSSHKeyForUsername("fake-username")
			Expect(err).ToNot(HaveOccurred())
			Expect(publicKey).To(Equal(""))
		})
	})

	Describe("Settings", func() {
		Context("when the settings file exists", func() {
			var (
				settingsFileName string
			)
			BeforeEach(func() {
				settingsFileName = "/fake-settings-file-path"
				source = infrastructure.NewFileSettingsSource(settingsFileName, fs, logger)
			})

			Context("settings have valid format", func() {
				var (
					expectedSettings boshsettings.Settings
				)

				BeforeEach(func() {
					expectedSettings = boshsettings.Settings{
						AgentID: "fake-agent-id",
					}

					settingsJSON, err := json.Marshal(expectedSettings)
					Expect(err).ToNot(HaveOccurred())
					err = fs.WriteFile(settingsFileName, settingsJSON)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns settings read from the file", func() {
					settings, err := source.Settings()
					Expect(err).ToNot(HaveOccurred())
					Expect(settings).To(Equal(expectedSettings))
				})

				It("should read from the file quietly", func() {
					_, err := source.Settings()
					Expect(err).ToNot(HaveOccurred())
					Expect(fs.ReadFileWithOptsCallCount).To(Equal(1))
				})
			})

			Context("settings have invalid format", func() {
				BeforeEach(func() {
					err := fs.WriteFileString(settingsFileName, "bad-json")
					Expect(err).NotTo(HaveOccurred())
				})
				It("returns settings read from the file", func() {
					_, err := source.Settings()
					Expect(err).To(HaveOccurred())
				})
			})

		})

		Context("when the registry file does not exist", func() {
			BeforeEach(func() {
				source = infrastructure.NewFileSettingsSource(
					"/missing-settings-file-path",
					fs, logger)
			})

			It("returns an error", func() {
				_, err := source.Settings()
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
