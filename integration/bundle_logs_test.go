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
		// The It below creates the "username" user via an ssh setup action but the agent's useradd
		// is not idempotent - it fails with "already exists" (exit 9) if the user is still present.
		// This spec used to leak that user (no cleanup), and since CI reuses the VM across suite runs
		// a leftover "username" from an earlier run made this spec flake. Remove it up front (and in
		// AfterEach), mirroring user_permissions_test, so the run is independent of prior state.
		testEnvironment.RunCommand("sudo userdel -rf username") //nolint:errcheck

		err := testEnvironment.UpdateAgentConfig(testEnvironment.GetSettingsFile(""))
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
		// Do not leak the "username" user created by the ssh setup action into later specs or, since
		// CI reuses the VM, into later suite runs (see BeforeEach).
		testEnvironment.RunCommand("sudo userdel -rf username") //nolint:errcheck

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
