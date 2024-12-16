package integration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("FileSettings", func() {
	Context("when vm is using file settings", func() {
		BeforeEach(func() {
			fileSettings := settings.Settings{
				AgentID: "fake-agent-id",
				Blobstore: settings.Blobstore{
					Type: "local",
					Options: map[string]interface{}{
						"blobstore_path": "/var/vcap/data",
					},
				},
				Disks: settings.Disks{
					Ephemeral: "/dev/sdh",
				},
			}

			err := testEnvironment.CreateSettingsFile(fileSettings)
			Expect(err).ToNot(HaveOccurred())

			err = testEnvironment.UpdateAgentConfig("file-settings-agent.json")
			Expect(err).ToNot(HaveOccurred())
			err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
			Expect(err).ToNot(HaveOccurred())

		})

		It("creates /var/vcap/bosh/settings.json", func() {
			var settingsJSON string
			var err error
			Eventually(func() error {
				settingsJSON, err = testEnvironment.GetFileContents("/var/vcap/bosh/settings.json")
				return err
			}).ShouldNot(HaveOccurred())
			Expect(settingsJSON).To(ContainSubstring("fake-agent-id"))
		})

		AfterEach(func() {
			err := testEnvironment.DetachDevice("/dev/sdh")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
