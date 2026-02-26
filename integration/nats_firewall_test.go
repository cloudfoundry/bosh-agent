package integration_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("nats firewall", func() {
	Context("ipv4", func() {
		BeforeEach(func() {
			// restore original settings of bosh from initial deploy of this VM.
			_, err := testEnvironment.RunCommand("sudo cp /settings-backup/*.json /var/vcap/bosh/")
			Expect(err).ToNot(HaveOccurred())
		})

		It("sets up the outgoing nats firewall", func() {
			format.MaxLength = 0

			Eventually(func() string {
				logs, _ := testEnvironment.RunCommand("sudo cat /var/vcap/bosh/log/current") //nolint:errcheck
				return logs
			}, 300).Should(ContainSubstring("Updated NATS firewall rules"))

			boshEnv := os.Getenv("BOSH_ENVIRONMENT")

			output, err := testEnvironment.RunCommand("sudo /home/agent_test_user/nftables-checker --table bosh_agent --chain nats_access")
			Expect(err).To(BeNil())
			Expect(output).To(MatchRegexp(`ACCEPT uid=0 dst=%s dport=4222`, boshEnv))
			Expect(output).To(MatchRegexp(`DROP dst=%s dport=4222`, boshEnv))

			// check that non-root cannot access the director nats, -w2 == timeout 2 seconds
			out, err := testEnvironment.RunCommand(fmt.Sprintf("nc %v 4222 -w2 -v", boshEnv))
			Expect(err).NotTo(BeNil())
			Expect(out).To(ContainSubstring("port 4222 (tcp) timed out"))

			// root (UID 0) is allowed through the nftables firewall
			out, err = testEnvironment.RunCommand(fmt.Sprintf("sudo nc %v 4222 -w2 -v", boshEnv))
			Expect(out).To(MatchRegexp("INFO.*server_id.*version.*host.*"))
			Expect(err).To(BeNil())
		})
	})

	Context("ipv6", func() {
		BeforeEach(func() {
			fileSettings := settings.Settings{
				AgentID: "fake-agent-id",
				Blobstore: settings.Blobstore{
					Type: "local",
					Options: map[string]interface{}{
						"blobstore_path": "/var/vcap/data",
					},
				},
				Mbus: "nats://[2001:db8::1]:4222",
				Disks: settings.Disks{
					Ephemeral: "/dev/sdh",
				},
			}

			err := testEnvironment.CreateSettingsFile(fileSettings)
			Expect(err).ToNot(HaveOccurred())
			err = testEnvironment.UpdateAgentConfig("file-settings-agent.json")
			Expect(err).ToNot(HaveOccurred())
			err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := testEnvironment.DetachDevice("/dev/sdh")
			Expect(err).ToNot(HaveOccurred())
			_, err = testEnvironment.RunCommand("sudo /home/agent_test_user/nftables-checker --table bosh_agent --chain nats_access --flush")
			Expect(err).To(BeNil())
		})

		It("sets up the outgoing nats for firewall ipv6", func() {
			format.MaxLength = 0

			Eventually(func() string {
				logs, _ := testEnvironment.RunCommand("sudo cat /var/vcap/bosh/log/current") //nolint:errcheck
				return logs
			}, 300).Should(ContainSubstring("Updated NATS firewall rules"))

			output, err := testEnvironment.RunCommand("sudo /home/agent_test_user/nftables-checker --table bosh_agent --chain nats_access")
			Expect(err).To(BeNil())
			Expect(output).To(ContainSubstring("ACCEPT uid=0 dst=2001:db8::1 dport=4222"))
			Expect(output).To(ContainSubstring("DROP dst=2001:db8::1 dport=4222"))
		})
	})
})
