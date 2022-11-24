package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("fetch_logs", func() {
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

	It("puts the logs in the appropriate blobstore location", func() {
		_, err := testEnvironment.RunCommand("echo 'foobarbaz' | sudo tee /var/vcap/sys/log/fetch-logs")
		Expect(err).NotTo(HaveOccurred())

		logsResponse, err := testEnvironment.AgentClient.FetchLogs("job", nil)
		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo zcat /var/vcap/data/blobs/%s", logsResponse["blobstore_id"]))

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring("foobarbaz"))
		Expect(output).To(ContainSubstring("fetch-logs"))
	})

})
