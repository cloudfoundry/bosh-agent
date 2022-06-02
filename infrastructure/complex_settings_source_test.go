package infrastructure_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("ComplexSettingsSource", func() {
	var (
		metadataService *fakeinf.FakeMetadataService
		source          ComplexSettingsSource
	)

	BeforeEach(func() {
		metadataService = &fakeinf.FakeMetadataService{}
		logger := boshlog.NewLogger(boshlog.LevelNone)
		source = NewComplexSettingsSource(metadataService, logger)
	})

	Describe("PublicSSHKeyForUsername", func() {
		It("returns an empty string", func() {
			metadataService.PublicKey = "public-key"

			publicKey, err := source.PublicSSHKeyForUsername("fake-username")
			Expect(err).ToNot(HaveOccurred())
			Expect(publicKey).To(Equal("public-key"))
		})

		It("returns an error if string", func() {
			metadataService.GetPublicKeyErr = errors.New("fake-public-key-error")

			_, err := source.PublicSSHKeyForUsername("fake-username")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-public-key-error"))
		})
	})

	Describe("Settings", func() {
		It("returns settings from the metadataService", func() {
			metadataService = &fakeinf.FakeMetadataService{
				Settings: boshsettings.Settings{
					AgentID: "fake-agent-id",
				},
			}
			logger := boshlog.NewLogger(boshlog.LevelNone)
			source = NewComplexSettingsSource(metadataService, logger)

			settings, err := source.Settings()
			Expect(err).ToNot(HaveOccurred())
			Expect(settings.AgentID).To(Equal("fake-agent-id"))
		})

		It("returns an error if getting settings fails", func() {
			metadataService = &fakeinf.FakeMetadataService{
				SettingsErr: errors.New("fake-get-settings-error"),
			}
			logger := boshlog.NewLogger(boshlog.LevelNone)
			source = NewComplexSettingsSource(metadataService, logger)

			_, err := source.Settings()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-get-settings-error"))
		})
	})
})
