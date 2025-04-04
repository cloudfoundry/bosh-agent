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

			// Wait a maximum of 300 seconds
			Eventually(func() string {
				logs, _ := testEnvironment.RunCommand("sudo cat /var/vcap/bosh/log/current") //nolint:errcheck
				return logs
			}, 300).Should(ContainSubstring("UbuntuNetManager"))

			output, err := testEnvironment.RunCommand("sudo iptables -t mangle -L")
			Expect(err).To(BeNil())
			// Check iptables for inclusion of the nats_cgroup_id
			Expect(output).To(MatchRegexp("ACCEPT *tcp  --  anywhere.*tcp dpt:4222 cgroup 2958295042"))
			Expect(output).To(MatchRegexp("DROP *tcp  --  anywhere.*tcp dpt:4222"))

			boshEnv := os.Getenv("BOSH_ENVIRONMENT")

			// check that we cannot access the director nats, -w2 == timeout 2 seconds
			out, err := testEnvironment.RunCommand(fmt.Sprintf("nc %v 4222 -w2 -v", boshEnv))
			Expect(err).NotTo(BeNil())
			Expect(out).To(ContainSubstring("port 4222 (tcp) timed out"))

			out, err = testEnvironment.RunCommand(fmt.Sprintf(`sudo sh -c '
            echo $$ >> $(cat /proc/self/mounts | grep ^cgroup | grep net_cls | cut -f2 -d" ")/nats-api-access/tasks
            nc %v 4222 -w2 -v'
		`, boshEnv))
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
				Mbus: "mbus://[2001:db8::1]:8080",
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

		It("sets up the outgoing nats for firewall  ipv6 ", func() {
			format.MaxLength = 0

			// Wait a maximum of 300 seconds
			Eventually(func() string {
				logs, _ := testEnvironment.RunCommand("sudo cat /var/vcap/bosh/log/current") //nolint:errcheck
				return logs
			}, 300).Should(ContainSubstring("UbuntuNetManager"))

			output, err := testEnvironment.RunCommand("sudo ip6tables -t mangle -L")
			Expect(err).To(BeNil())

			// Check iptables for inclusion of the nats_cgroup_id
			Expect(output).To(MatchRegexp("ACCEPT *tcp *anywhere *2001:db8::1 *tcp dpt:http-alt cgroup 2958295042"))
			Expect(output).To(MatchRegexp("DROP *tcp *anywhere *2001:db8::1 *tcp dpt:http-alt"))

			Expect(output).To(MatchRegexp("2001:db8::1"))

		})
		AfterEach(func() {
			err := testEnvironment.DetachDevice("/dev/sdh")
			Expect(err).ToNot(HaveOccurred())
			_, err = testEnvironment.RunCommand("sudo ip6tables -t mangle -D POSTROUTING -d 2001:db8::1 -p tcp --dport 8080 -m cgroup --cgroup 2958295042 -j ACCEPT --wait")
			Expect(err).To(BeNil())
			_, err = testEnvironment.RunCommand("sudo ip6tables -t mangle -D POSTROUTING -d 2001:db8::1 -p tcp --dport 8080 -j DROP --wait")
			Expect(err).To(BeNil())
		})
	})
})
