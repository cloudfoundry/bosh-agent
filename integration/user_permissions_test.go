package integration_test

import (
	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	"github.com/cloudfoundry/bosh-agent/v2/settings"

	"strings"

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

		err = testEnvironment.CleanupSSH()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.UpdateAgentConfig("file-settings-agent.json")
		Expect(err).ToNot(HaveOccurred())

		networks, err := testEnvironment.GetVMNetworks()
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
			Networks: networks,
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

	JustBeforeEach(func() {
		err := testEnvironment.StartAgentTunnel()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("on ubuntu when a new user is created", func() {
		BeforeEach(func() {
			_, err := testEnvironment.RunCommand("sudo groupadd -f bosh_sudoers")
			Expect(err).ToNot(HaveOccurred())
			_, err = testEnvironment.RunCommand("sudo groupadd -f bosh_sshers")
			Expect(err).ToNot(HaveOccurred())
			testEnvironment.RunCommand("sudo userdel -rf username") //nolint:errcheck
		})

		AfterEach(func() {
			testEnvironment.RunCommand("sudo userdel -rf username") //nolint:errcheck
		})

		It("should contain the correct home directory permissions", func() {
			err := testEnvironment.AgentClient.SSH("setup", action.SSHParams{
				User:      "username",
				PublicKey: "public-key",
			})

			Expect(err).ToNot(HaveOccurred())

			verifyFilePerm("755", "/var/vcap/bosh_ssh", testEnvironment)
			verifyFilePerm("700", "/var/vcap/bosh_ssh/username", testEnvironment)
		})
	})
})

func verifyFilePerm(perm string, filePath string, testEnvironment *integration.TestEnvironment) {
	filePerms, err := testEnvironment.RunCommand("sudo stat -c '%a %n' " + filePath + " | cut -d' ' -f 1")
	Expect(err).NotTo(HaveOccurred())

	Expect(strings.Trim(filePerms, "\n")).To(Equal(perm))
}
