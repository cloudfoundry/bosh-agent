package infrastructure_test

import (
	"encoding/json"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	fakedpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver/fakes"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	fakefs "github.com/cloudfoundry/bosh-agent/system/fakes"
)

var _ = Describe("dummyInfrastructure", func() {
	Describe("GetSettings", func() {
		var (
			fs          *fakefs.FakeFileSystem
			dirProvider boshdir.Provider
			inf         Infrastructure
		)

		BeforeEach(func() {
			fs = fakefs.NewFakeFileSystem()
			dirProvider = boshdir.NewProvider("/var/vcap")
			platform := fakeplatform.NewFakePlatform()
			fakeDevicePathResolver := fakedpresolv.NewFakeDevicePathResolver()
			inf = NewDummyInfrastructure(fs, dirProvider, platform, fakeDevicePathResolver)
		})

		Context("when infrastructure settings file is found", func() {
			BeforeEach(func() {
				settingsPath := filepath.Join(dirProvider.BoshDir(), "dummy-cpi-agent-env.json")

				expectedSettings := boshsettings.Settings{
					AgentID: "123-456-789",
					Blobstore: boshsettings.Blobstore{
						Type: boshsettings.BlobstoreTypeDummy,
					},
					Mbus: "nats://127.0.0.1:4222",
				}
				existingSettingsBytes, err := json.Marshal(expectedSettings)
				Expect(err).ToNot(HaveOccurred())

				fs.WriteFile(settingsPath, existingSettingsBytes)
			})

			It("returns settings", func() {
				settings, err := inf.GetSettings()
				Expect(err).ToNot(HaveOccurred())
				assert.Equal(GinkgoT(), settings, boshsettings.Settings{
					AgentID:   "123-456-789",
					Blobstore: boshsettings.Blobstore{Type: boshsettings.BlobstoreTypeDummy},
					Mbus:      "nats://127.0.0.1:4222",
				})
			})
		})

		Context("when infrastructure settings file is not found", func() {
			It("returns error", func() {
				_, err := inf.GetSettings()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Read settings file"))
			})
		})
	})
})
