package infrastructure_test

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
)

var _ = Describe("ConfigDriveSettingsSource", func() {
	var (
		platform *fakeplatform.FakePlatform
		source   *ConfigDriveSettingsSource
	)

	BeforeEach(func() {
		diskPaths := []string{"/fake-disk-path"}
		metadataPath := "fake-metadata-path"
		settingsPath := "fake-settings-path"
		platform = fakeplatform.NewFakePlatform()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		source = NewConfigDriveSettingsSource(diskPaths, metadataPath, settingsPath, platform, logger)
	})

	BeforeEach(func() {
		// Set up default settings and metadata
		platform.SetGetFilesContentsFromDisk("fake-metadata-path", []byte(`{}`), nil)
		platform.SetGetFilesContentsFromDisk("fake-settings-path", []byte(`{}`), nil)
	})

	Describe("PublicSSHKeyForUsername", func() {
		Context("when metadata contains a public SSH key", func() {
			metadata := MetadataContentsType{
				PublicKeys: map[string]PublicKeyType{
					"0": PublicKeyType{
						"openssh-key": "fake-openssh-key",
					},
				},
			}

			It("returns public key from the config drive", func() {
				metadataBytes, err := json.Marshal(metadata)
				Expect(err).ToNot(HaveOccurred())

				platform.SetGetFilesContentsFromDisk("fake-metadata-path", metadataBytes, nil)

				publicKey, err := source.PublicSSHKeyForUsername("fake-username")
				Expect(err).ToNot(HaveOccurred())
				Expect(publicKey).To(Equal("fake-openssh-key"))
			})

			It("returns an error if getting public SSH key fails", func() {
				platform.SetGetFilesContentsFromDisk(
					"fake-metadata-path", []byte{}, errors.New("fake-read-disk-error"))

				publicKey, err := source.PublicSSHKeyForUsername("fake-username")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-read-disk-error"))

				Expect(publicKey).To(Equal(""))
			})

			It("does not try to read settings from the config drive more than once", func() {
				metadataBytes, err := json.Marshal(metadata)
				Expect(err).ToNot(HaveOccurred())

				platform.SetGetFilesContentsFromDisk("fake-metadata-path", metadataBytes, nil)

				publicKey, err := source.PublicSSHKeyForUsername("fake-username")
				Expect(err).ToNot(HaveOccurred())
				Expect(publicKey).To(Equal("fake-openssh-key"))

				publicKey, err = source.PublicSSHKeyForUsername("fake-username")
				Expect(err).ToNot(HaveOccurred())
				Expect(publicKey).To(Equal("fake-openssh-key"))

				Expect(platform.GetFileContentsFromDiskCalledTimes).To(Equal(1))
			})
		})

		Context("when metadata does not contain a public SSH key", func() {
			metadata := MetadataContentsType{}

			It("returns an empty string", func() {
				metadataBytes, err := json.Marshal(metadata)
				Expect(err).ToNot(HaveOccurred())

				platform.SetGetFilesContentsFromDisk("fake-metadata-path", metadataBytes, nil)

				publicKey, err := source.PublicSSHKeyForUsername("fake-username")
				Expect(err).ToNot(HaveOccurred())
				Expect(publicKey).To(Equal(""))
			})
		})
	})

	Describe("Settings", func() {
		It("returns settings read from the config drive", func() {
			platform.SetGetFilesContentsFromDisk(
				"fake-settings-path", []byte(`{"agent_id": "123"}`), nil)

			settings, err := source.Settings()
			Expect(err).ToNot(HaveOccurred())

			Expect(platform.GetFileContentsFromDiskDiskPaths).To(Equal([]string{"/fake-disk-path"}))
			Expect(settings.AgentID).To(Equal("123"))
		})

		It("returns an error if reading from the config drive fails", func() {
			platform.SetGetFilesContentsFromDisk(
				"fake-settings-path", []byte{}, errors.New("fake-read-disk-error"))

			_, err := source.Settings()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-read-disk-error"))
		})

<<<<<<< HEAD
		It("does not try to read settings from the config drive more than once", func() {
			platform.SetGetFilesContentsFromDisk(
				"fake-settings-path", []byte(`{"agent_id": "123"}`), nil)

			settings, err := source.Settings()
			Expect(err).ToNot(HaveOccurred())
			Expect(settings.AgentID).To(Equal("123"))

			settings, err = source.Settings()
			Expect(err).ToNot(HaveOccurred())
			Expect(settings.AgentID).To(Equal("123"))

			Expect(platform.GetFileContentsFromDiskCalledTimes).To(Equal(1))
		})
	})
})
