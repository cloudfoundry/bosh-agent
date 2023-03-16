package integration_test

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/integration/integrationagentclient"
	"github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("fetch_logs_with_signed_url", func() {
	var (
		agentClient      *integrationagentclient.IntegrationAgentClient
		registrySettings settings.Settings
	)

	BeforeEach(func() {
		err := testEnvironment.StopAgent()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupLogFile()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.SetupConfigDrive()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.UpdateAgentConfig("config-drive-agent.json")
		Expect(err).ToNot(HaveOccurred())

		registrySettings = settings.Settings{
			AgentID: "fake-agent-id",

			// note that this SETS the username and password for HTTP message bus access
			Mbus: "https://mbus-user:mbus-pass@127.0.0.1:6868",

			Blobstore: settings.Blobstore{
				Type: "local",
				Options: map[string]interface{}{
					"blobstore_path": "/var/vcap/data/blobs",
				},
			},

			Disks: settings.Disks{
				Ephemeral: "/dev/sdh",
			},

			Networks: map[string]settings.Network{
				"default": settings.Network{
					UseDHCP: true,
					DNS:     []string{"8.8.8.8"},
				},
			},
		}

		err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StartRegistry(registrySettings)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StartAgent()
		Expect(err).ToNot(HaveOccurred())

		agentClient, err = testEnvironment.StartAgentTunnel("mbus-user", "mbus-pass", 6868)
		Expect(err).NotTo(HaveOccurred())

		_, err = testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data")
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.StartBlobstore()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := testEnvironment.StopAgentTunnel()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.StopAgent()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	It("job log fetch works", func() {
		_, err := testEnvironment.RunCommand("echo 'foobarbaz-job-log-fetch' | sudo tee /var/vcap/sys/log/fetch-logs-job")
		Expect(err).NotTo(HaveOccurred())

		signedURL := "http://127.0.0.1:9091/upload_package/logs.tgz"

		_, err = agentClient.FetchLogsWithSignedURLAction(signedURL, "job", nil)
		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo zcat %s", filepath.Join(testEnvironment.BlobstoreDir(), "logs.tgz")))
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring("foobarbaz-job-log-fetch"))
		Expect(output).To(ContainSubstring("fetch-logs-job"))
	})

	It("agent log fetch works", func() {
		_, err := testEnvironment.RunCommand("echo 'foobarbaz-agent-log-fetch' | sudo tee /var/vcap/bosh/log/fetch-logs-agent")
		Expect(err).NotTo(HaveOccurred())

		signedURL := "http://127.0.0.1:9091/upload_package/logs.tgz"

		_, err = agentClient.FetchLogsWithSignedURLAction(signedURL, "agent", nil)
		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo zcat %s", filepath.Join(testEnvironment.BlobstoreDir(), "logs.tgz")))
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring("foobarbaz-agent-log-fetch"))
		Expect(output).To(ContainSubstring("fetch-logs-agent"))
	})

	It("system log fetch works", func() {
		_, err := testEnvironment.RunCommand("echo 'foobarbaz-system-log-fetch' | sudo tee /var/log/fetch-logs-system")
		Expect(err).NotTo(HaveOccurred())

		signedURL := "http://127.0.0.1:9091/upload_package/logs.tgz"

		_, err = agentClient.FetchLogsWithSignedURLAction(signedURL, "system", nil)
		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo zcat %s", filepath.Join(testEnvironment.BlobstoreDir(), "logs.tgz")))
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring("foobarbaz-system-log-fetch"))
		Expect(output).To(ContainSubstring("fetch-logs-system"))
	})

})
