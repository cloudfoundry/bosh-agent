package integration_test

import (
	"github.com/cloudfoundry/bosh-agent/v2/settings"

	"github.com/cloudfoundry/bosh-agent/v2/agentclient/applyspec"
	"github.com/cloudfoundry/bosh-agent/v2/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Instance Info", func() {
	var (
		fileSettings settings.Settings
	)

	BeforeEach(func() {
		err := testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupLogFile()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.UpdateAgentConfig("file-settings-agent.json")
		Expect(err).ToNot(HaveOccurred())

		fileSettings = settings.Settings{
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

		err = testEnvironment.CreateSettingsFile(fileSettings)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	Context("on ubuntu", func() {

		JustBeforeEach(func() {
			err := testEnvironment.StartAgentTunnel()
			Expect(err).NotTo(HaveOccurred())
		})

		It("apply spec saves instance info to file and is readable by anyone", func() {
			applySpec := applyspec.ApplySpec{ConfigurationHash: "fake-desired-config-hash", NodeID: "node-id01-123f-r2344", AvailabilityZone: "ex-az", Deployment: "deployment-name", Name: "instance-name"}
			err := testEnvironment.AgentClient.Apply(applySpec)
			Expect(err).NotTo(HaveOccurred())

			verifyFilePermissions("/var/vcap/instance/id", testEnvironment)
			verifyFileContent("/var/vcap/instance/id", applySpec.NodeID, testEnvironment)

			verifyFilePermissions("/var/vcap/instance/az", testEnvironment)
			verifyFileContent("/var/vcap/instance/az", applySpec.AvailabilityZone, testEnvironment)

			verifyFilePermissions("/var/vcap/instance/name", testEnvironment)
			verifyFileContent("/var/vcap/instance/name", applySpec.Name, testEnvironment)

			verifyFilePermissions("/var/vcap/instance/deployment", testEnvironment)
			verifyFileContent("/var/vcap/instance/deployment", applySpec.Deployment, testEnvironment)

			verifyDirectoryExecutable("/var/vcap/instance", testEnvironment)
		})
	})
})

func verifyFileContent(filePath string, expectedContent string, testEnvironment *integration.TestEnvironment) {
	deployment, err := testEnvironment.RunCommand("cat " + filePath)
	Expect(err).NotTo(HaveOccurred())
	Expect(deployment).To(Equal(expectedContent))
}

func verifyFilePermissions(filePath string, testEnvironment *integration.TestEnvironment) {
	fileListing, err := testEnvironment.RunCommand("ls -l " + filePath)
	Expect(err).NotTo(HaveOccurred())

	Expect(fileListing[1]).To(Equal(uint8('r')))
	Expect(fileListing[2]).To(Equal(uint8('w')))
	Expect(fileListing[4]).To(Equal(uint8('r')))
	Expect(fileListing[7]).To(Equal(uint8('r')))
}

func verifyDirectoryExecutable(filePath string, testEnvironment *integration.TestEnvironment) {
	fileListing, err := testEnvironment.RunCommand("ls -l -d " + filePath)
	Expect(err).NotTo(HaveOccurred())

	Expect(fileListing[3]).To(Equal(uint8('x')))
	Expect(fileListing[6]).To(Equal(uint8('x')))
	Expect(fileListing[9]).To(Equal(uint8('x')))
}
