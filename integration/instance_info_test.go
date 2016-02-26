package integration_test

import (
	"github.com/cloudfoundry/bosh-agent/agentclient"
	"github.com/cloudfoundry/bosh-agent/settings"

	"github.com/cloudfoundry/bosh-agent/agentclient/applyspec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Instance Info", func() {
	var (
		agentClient      agentclient.AgentClient
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
					"blobstore_path": "/var/vcap/data",
				},
			},

			Disks: settings.Disks{
				Ephemeral: "/dev/sdh",
			},
		}

		err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StartRegistry(registrySettings)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		err := testEnvironment.StartAgent()
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

	Context("on ubuntu", func() {
		It("apply spec saves instance info to file and is readable by anyone", func() {
			applySpec := applyspec.ApplySpec{ConfigurationHash: "fake-desired-config-hash", NodeID: "node-id01-123f-r2344", AvailabilityZone: "ex-az", Deployment: "deployment-name"}
			err := agentClient.Apply(applySpec)

			Expect(err).NotTo(HaveOccurred())

			id, err := testEnvironment.RunCommand("cat /var/vcap/bosh/etc/instance/id")
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal(applySpec.NodeID))

			idListing, err := testEnvironment.RunCommand("ls -l /var/vcap/bosh/etc/instance/id")
			Expect(err).NotTo(HaveOccurred())
			Expect(idListing[1]).To(Equal(uint8('r')))
			Expect(idListing[4]).To(Equal(uint8('r')))
			Expect(idListing[7]).To(Equal(uint8('r')))

			az, err := testEnvironment.RunCommand("cat /var/vcap/bosh/etc/instance/az")
			Expect(err).NotTo(HaveOccurred())
			Expect(az).To(Equal(applySpec.AvailabilityZone))

			azListing, err := testEnvironment.RunCommand("ls -l /var/vcap/bosh/etc/instance/az")
			Expect(err).NotTo(HaveOccurred())
			Expect(azListing[1]).To(Equal(uint8('r')))
			Expect(azListing[4]).To(Equal(uint8('r')))
			Expect(azListing[7]).To(Equal(uint8('r')))

			name, err := testEnvironment.RunCommand("cat /var/vcap/bosh/etc/instance/name")
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal(applySpec.Deployment))

			nameListing, err := testEnvironment.RunCommand("ls -l /var/vcap/bosh/etc/instance/name")
			Expect(err).NotTo(HaveOccurred())
			Expect(nameListing[1]).To(Equal(uint8('r')))
			Expect(nameListing[4]).To(Equal(uint8('r')))
			Expect(nameListing[7]).To(Equal(uint8('r')))
		})
	})
})
