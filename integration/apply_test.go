package integration_test

import (
	"github.com/cloudfoundry/bosh-agent/agentclient"
	. "github.com/cloudfoundry/bosh-agent/agentclient/applyspec"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("TestApply", func() {
	var (
		registrySettings boshsettings.Settings
		agentClient      agentclient.AgentClient
		applySpec        ApplySpec
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

		registrySettings = boshsettings.Settings{
			AgentID: "fake-agent-id",

			Mbus: "https://mbus-user:mbus-pass@127.0.0.1:6868",

			Blobstore: boshsettings.Blobstore{
				Type: "local",
				Options: map[string]interface{}{
					"blobstore_path": "/var/vcap/data",
				},
			},

			Disks: boshsettings.Disks{
				Ephemeral: "/dev/sdh",
			},
		}

		err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StartRegistry(registrySettings)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when apply is called", func() {
		BeforeEach(func() {
			err := testEnvironment.StartAgent()
			Expect(err).ToNot(HaveOccurred())

			agentClient, err = testEnvironment.StartAgentTunnel("mbus-user", "mbus-pass", 6868)
			Expect(err).NotTo(HaveOccurred())

			applySpec = ApplySpec{
				Deployment:        "fake-deployment-name",
				Index:             0,
				ConfigurationHash: "some-configuration-hash",
			}
		})

		AfterEach(func() {
			err := testEnvironment.StopAgentTunnel()
			Expect(err).NotTo(HaveOccurred())

			err = testEnvironment.StopAgent()
			Expect(err).ToNot(HaveOccurred())

			err = testEnvironment.DetachDevice("/dev/sdh")
			Expect(err).ToNot(HaveOccurred())
		})

		It("stops monit service", func() {
			err := agentClient.Apply(applySpec)
			Expect(err).To(BeNil())

			result, _ := testEnvironment.RunCommand("sudo sv status monit")

			Expect(result).To(ContainSubstring("down: monit: "))
		})
	})
})
