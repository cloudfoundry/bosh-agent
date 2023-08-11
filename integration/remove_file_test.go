package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("remove_file", func() {
	var (
		fileSettings settings.Settings
	)

	BeforeEach(func() {
		err := testEnvironment.UpdateAgentConfig("file-settings-agent.json")
		Expect(err).ToNot(HaveOccurred())

		fileSettings = settings.Settings{
			Blobstore: settings.Blobstore{
				Type: "local",
				Options: map[string]interface{}{
					"blobstore_path": "/var/vcap/data/blobs",
				},
			},

			Networks: settings.Networks{
				"fake-net": settings.Network{IP: "127.0.0.1"},
			},

			Disks: settings.Disks{
				Ephemeral: "/dev/sdh",
			},
		}

		err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CreateFilesettings(fileSettings)
		Expect(err).ToNot(HaveOccurred())

		_, err = testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data")
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		err := testEnvironment.StartAgentTunnel()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	It("removes the specified file", func() {
		_, err := testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data/tmp")
		Expect(err).NotTo(HaveOccurred())

		const tempFile string = "/var/vcap/data/tmp/foo"
		_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo touch %s", tempFile))
		Expect(err).NotTo(HaveOccurred())

		fileStat, err := testEnvironment.RunCommand(fmt.Sprintf("sudo stat %s", tempFile))
		Expect(err).NotTo(HaveOccurred())
		Expect(fileStat).To(ContainSubstring(tempFile))

		err = testEnvironment.AgentClient.RemoveFile(tempFile)
		Expect(err).NotTo(HaveOccurred())

		fileStat, err = testEnvironment.RunCommand(fmt.Sprintf("sudo stat %s || true", tempFile))
		Expect(err).NotTo(HaveOccurred())
		Expect(fileStat).To(ContainSubstring("No such file"))
	})

})
