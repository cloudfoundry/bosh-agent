package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	"github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("bundle_logs", func() {
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

	It("puts the logs in the appropriate location", func() {
		_, err := testEnvironment.RunCommand("echo 'foobarbaz' | sudo tee /var/vcap/sys/log/bundle-logs")
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.AgentClient.SSH("setup", action.SSHParams{
			User:      "username",
			PublicKey: "public-key",
		})
		Expect(err).ToNot(HaveOccurred())

		logsResponse, err := testEnvironment.AgentClient.BundleLogs("username", "job", []string{})
		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo zcat %s", logsResponse.LogsTarPath))

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring("foobarbaz"))
		Expect(output).To(ContainSubstring("bundle-logs"))

		fileStat, err := testEnvironment.RunCommand("sudo stat -c '%a %G %U' " + logsResponse.LogsTarPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(fileStat).To(ContainSubstring("600 username username"))
	})

})
