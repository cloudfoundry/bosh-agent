package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/integration/integrationagentclient"
	"github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("remove_file", func() {
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

			Networks: settings.Networks{
				"fake-net": settings.Network{IP: "127.0.0.1"},
			},

			Disks: settings.Disks{
				Ephemeral: "/dev/sdh",
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
	})

	AfterEach(func() {
		err := testEnvironment.StopAgentTunnel()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.StopAgent()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	FIt("removes the specified file", func() {
		_, err := testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data/tmp")
		Expect(err).NotTo(HaveOccurred())

		const tempFile string = "/var/vcap/data/tmp/foo"
		_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo touch %s", tempFile))
		Expect(err).NotTo(HaveOccurred())

		fileStat, err := testEnvironment.RunCommand(fmt.Sprintf("sudo stat %s", tempFile))
		Expect(err).NotTo(HaveOccurred())
		Expect(fileStat).To(ContainSubstring(tempFile))

		err = agentClient.RemoveFile(tempFile)
		Expect(err).NotTo(HaveOccurred())

		fileStat, err = testEnvironment.RunCommand(fmt.Sprintf("sudo stat %s || true", tempFile))
		Expect(err).NotTo(HaveOccurred())
		Expect(fileStat).To(ContainSubstring("No such file"))
	})

})
