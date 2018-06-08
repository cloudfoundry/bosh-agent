package infrastructure_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("ConfigDriveSettingsSource", func() {
	var (
		platform *platformfakes.FakePlatform
		source   *CDROMSettingsSource
	)

	BeforeEach(func() {
		settingsFileName := "fake-settings-file-name"
		platform = &platformfakes.FakePlatform{}
		logger := boshlog.NewLogger(boshlog.LevelNone)
		source = NewCDROMSettingsSource(settingsFileName, platform, logger)
	})

	Describe("PublicSSHKeyForUsername", func() {
		It("returns an empty string", func() {
			publicKey, err := source.PublicSSHKeyForUsername("fake-username")
			Expect(err).ToNot(HaveOccurred())
			Expect(publicKey).To(Equal(""))
		})
	})

	Describe("Settings", func() {
		It("returns settings read from the CDROM", func() {
			platform.GetFileContentsFromCDROMReturns([]byte(`{"agent_id": "123"}`), nil)

			settings, err := source.Settings()
			Expect(err).ToNot(HaveOccurred())

			Expect(platform.GetFileContentsFromCDROMCallCount()).To(Equal(1))
			Expect(platform.GetFileContentsFromCDROMArgsForCall(0)).To(Equal("fake-settings-file-name"))
			Expect(settings.AgentID).To(Equal("123"))
		})

		It("returns an error if reading from the CDROM fails", func() {
			platform.GetFileContentsFromCDROMReturns([]byte(`{"agent_id": "123"}`), errors.New("fake-read-disk-error"))

			_, err := source.Settings()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-read-disk-error"))
		})
	})
})
