package integration_test

import (
	"os"

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
	})

	AfterEach(func() {
		err := testEnvironment.StopAgentTunnel()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.StopAgent()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	var (
		bucket string
		key    string
	)

	BeforeEach(func() {
		bucket = os.Getenv("AWS_BUCKET")
		key = "s3-signed-file.txt"
	})

	AfterEach(func() {
		removeS3Object(bucket, key)
	})

	It("puts the logs in the appropriate blobstore location", func() {
		r, stderr, _, err := testEnvironment.RunCommand3("echo 'foobarbaz' | sudo tee /var/vcap/sys/log/fetch-logs")
		Expect(err).NotTo(HaveOccurred(), r, stderr)

		signedURL := generateSignedURLForPut(bucket, key)

		response, err := agentClient.FetchLogsWithSignedURLAction(signedURL, "job", nil)
		Expect(err).NotTo(HaveOccurred())

		contents, sha1 := downloadS3ObjectContents(bucket, key)
		Expect(response.SHA1Digest).To(Equal(sha1))

		Expect(contents).To(ContainSubstring("foobarbaz"))
		Expect(contents).To(ContainSubstring("fetch-logs"))
	})
})
