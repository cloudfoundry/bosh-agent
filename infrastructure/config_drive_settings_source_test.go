package infrastructure_test

import (
	"encoding/json"
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
		source   *ConfigDriveSettingsSource
	)

	BeforeEach(func() {
		diskPaths := []string{"/fake-disk-path-1", "/fake-disk-path-2"}
		metadataPath := "fake-metadata-path"
		settingsPath := "fake-settings-path"
		platform = &platformfakes.FakePlatform{}
		logger := boshlog.NewLogger(boshlog.LevelNone)
		source = NewConfigDriveSettingsSource(diskPaths, metadataPath, settingsPath, platform, logger)
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

				platform.GetFilesContentsFromDiskReturns([][]byte{metadataBytes}, nil)

				publicKey, err := source.PublicSSHKeyForUsername("fake-username")
				Expect(err).ToNot(HaveOccurred())
				Expect(publicKey).To(Equal("fake-openssh-key"))
			})

			It("errors when there are no keys on any of the config drives", func() {
				platform.GetFilesContentsFromDiskReturnsOnCall(0, [][]byte{[]byte{}}, errors.New("fake-read-disk-error-1"))
				platform.GetFilesContentsFromDiskReturnsOnCall(1, [][]byte{[]byte{}}, errors.New("fake-read-disk-error-2"))

				publicKey, err := source.PublicSSHKeyForUsername("fake-username")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-read-disk-error-2"))

				Expect(publicKey).To(Equal(""))
			})
		})

		Context("when metadata does not contain a public SSH key", func() {
			metadata := MetadataContentsType{}

			It("returns an empty string", func() {
				metadataBytes, err := json.Marshal(metadata)
				Expect(err).ToNot(HaveOccurred())

				platform.GetFilesContentsFromDiskReturns([][]byte{metadataBytes}, nil)

				publicKey, err := source.PublicSSHKeyForUsername("fake-username")
				Expect(err).ToNot(HaveOccurred())
				Expect(publicKey).To(Equal(""))
			})
		})
	})

	Describe("Settings", func() {
		It("returns settings read from the config drive", func() {
			platform.GetFilesContentsFromDiskReturns([][]byte{[]byte(`{"agent_id": "123"}`)}, nil)
			settings, err := source.Settings()
			Expect(err).ToNot(HaveOccurred())

			diskPath, _ := platform.GetFilesContentsFromDiskArgsForCall(0)
			Expect(diskPath).To(Equal("/fake-disk-path-1"))
			Expect(settings.AgentID).To(Equal("123"))
		})

		It("tries to load settings from multiple potential disk locations", func() {
			platform.GetFilesContentsFromDiskReturnsOnCall(0, [][]byte{[]byte{}}, errors.New("fake-read-disk-error"))
			platform.GetFilesContentsFromDiskReturnsOnCall(1, [][]byte{[]byte(`{"agent_id": "123"}`)}, nil)

			settings, err := source.Settings()
			Expect(err).ToNot(HaveOccurred())
			Expect(settings.AgentID).To(Equal("123"))

			diskPath, _ := platform.GetFilesContentsFromDiskArgsForCall(0)
			Expect(diskPath).To(Equal("/fake-disk-path-1"))
			diskPath, _ = platform.GetFilesContentsFromDiskArgsForCall(1)
			Expect(diskPath).To(Equal("/fake-disk-path-2"))
		})

		It("returns an error if reading from potential disk paths for config drive fails", func() {
			platform.GetFilesContentsFromDiskReturnsOnCall(0, [][]byte{[]byte{}}, errors.New("fake-read-disk-error-1"))
			platform.GetFilesContentsFromDiskReturnsOnCall(1, [][]byte{[]byte{}}, errors.New("fake-read-disk-error-2"))
			_, err := source.Settings()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-read-disk-error-2"))
		})
	})
})
