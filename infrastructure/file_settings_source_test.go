package infrastructure_test

import (
	"encoding/json"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	fakesys "github.com/cloudfoundry/bosh-agent/system/fakes"
)

var _ = Describe("FileSettingsSource", func() {
	var (
		fs     *fakesys.FakeFileSystem
		source *FileSettingsSource
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
				expectedSettings boshsettings.Settings
			)

			BeforeEach(func() {
				settingsFileName := "/fake-settings-file-path"
				expectedSettings = boshsettings.Settings{
					AgentID: "fake-agent-id",
				}

				settingsJSON, err := json.Marshal(expectedSettings)
				Expect(err).ToNot(HaveOccurred())
				fs.WriteFile(settingsFileName, settingsJSON)

				source = NewFileSettingsSource(
					settingsFileName,
					fs, logger)
			})

			It("returns settings read from the file", func() {
				settings, err := source.Settings()
				Expect(err).ToNot(HaveOccurred())
				Expect(settings).To(Equal(expectedSettings))
			})
		})

		Context("when the registry file does not exist", func() {

			BeforeEach(func() {
				source = NewFileSettingsSource(
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
