package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
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

		err = testEnvironment.CreateSettingsFile(fileSettings)
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

	It("job log fetch works", func() {
		_, err := testEnvironment.RunCommand("echo 'foobarbaz-job-log-fetch' | sudo tee /var/vcap/sys/log/fetch-logs-job")
		Expect(err).NotTo(HaveOccurred())

		logsResponse, err := testEnvironment.AgentClient.FetchLogs("job", nil)
		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo zcat /var/vcap/data/blobs/%s", logsResponse["blobstore_id"]))

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring("foobarbaz-job-log-fetch"))
		Expect(output).To(ContainSubstring("fetch-logs-job"))
	})

	It("agent log fetch works", func() {
		_, err := testEnvironment.RunCommand("echo 'foobarbaz-agent-log-fetch' | sudo tee /var/vcap/bosh/log/fetch-logs-agent")
		Expect(err).NotTo(HaveOccurred())

		logsResponse, err := testEnvironment.AgentClient.FetchLogs("agent", nil)
		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo zcat /var/vcap/data/blobs/%s", logsResponse["blobstore_id"]))

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring("foobarbaz-agent-log-fetch"))
		Expect(output).To(ContainSubstring("fetch-logs-agent"))
	})

	It("system log fetch works", func() {
		_, err := testEnvironment.RunCommand("echo 'foobarbaz-system-log-fetch' | sudo tee /var/log/fetch-logs-system")
		Expect(err).NotTo(HaveOccurred())

		logsResponse, err := testEnvironment.AgentClient.FetchLogs("system", nil)
		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo zcat /var/vcap/data/blobs/%s", logsResponse["blobstore_id"]))

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring("foobarbaz-system-log-fetch"))
		Expect(output).To(ContainSubstring("fetch-logs-system"))
	})

})
