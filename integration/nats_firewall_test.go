package integration_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

var _ = Describe("nats firewall", func() {

	Context("nftables ipv4", func() {
		BeforeEach(func() {
			// restore original settings of bosh from initial deploy of this VM.
			_, err := testEnvironment.RunCommand("sudo cp /settings-backup/*.json /var/vcap/bosh/")
			Expect(err).ToNot(HaveOccurred())
		})

		It("sets up the outgoing nats firewall using nftables", func() {
			format.MaxLength = 0

			// Wait for the agent to start and set up firewall rules
			Eventually(func() string {
				logs, _ := testEnvironment.RunCommand("sudo cat /var/vcap/bosh/log/current") //nolint:errcheck
				return logs
			}, 300).Should(ContainSubstring("NftablesFirewall"))

			// Check nftables for the bosh_agent table and chains
			output, err := testEnvironment.RunCommand("sudo nft list table inet bosh_agent")
			Expect(err).To(BeNil())

			// Verify table structure - should have both monit_access and nats_access chains
			Expect(output).To(ContainSubstring("table inet bosh_agent"))
			Expect(output).To(ContainSubstring("chain monit_access"))
			Expect(output).To(ContainSubstring("chain nats_access"))

			// Verify monit rules - classid 0xb0540001 (2958295041) for port 2822
			// The output format is: socket cgroupv2 level X classid <hex_value>
			Expect(output).To(MatchRegexp("tcp dport 2822.*socket cgroupv2.*classid.*accept"))

			// Verify NATS rules - classid 0xb0540002 (2958295042) for the NATS port
			// The NATS chain should have rules for the director's NATS port (usually 4222)
			Expect(output).To(MatchRegexp("nats_access.*tcp dport"))

			boshEnv := os.Getenv("BOSH_ENVIRONMENT")

			// Test that we cannot access the director nats from outside the agent cgroup
			// -w2 == timeout 2 seconds
			out, err := testEnvironment.RunCommand(fmt.Sprintf("nc %v 4222 -w2 -v", boshEnv))
			Expect(err).NotTo(BeNil())
			Expect(out).To(ContainSubstring("timed out"))

			// Test that we CAN access NATS when running in the agent's cgroup
			// Find the agent's cgroup path and run nc from within it
			out, err = testEnvironment.RunCommand(fmt.Sprintf(`sudo sh -c '
				# Find the agent process cgroup
				agent_pid=$(pgrep -f "bosh-agent$" | head -1)
				if [ -z "$agent_pid" ]; then
					echo "Agent process not found"
					exit 1
				fi
				# For cgroup v2, add ourselves to the same cgroup as the agent
				agent_cgroup=$(cat /proc/$agent_pid/cgroup | head -1 | cut -d: -f3)
				echo $$ > /sys/fs/cgroup${agent_cgroup}/cgroup.procs
				nc %v 4222 -w2 -v'
			`, boshEnv))
			Expect(err).To(BeNil())
			Expect(out).To(MatchRegexp("INFO.*server_id.*version.*host.*"))
		})
	})

	Context("nftables ipv6", func() {
		BeforeEach(func() {
			// restore original settings of bosh from initial deploy of this VM.
			_, err := testEnvironment.RunCommand("sudo cp /settings-backup/*.json /var/vcap/bosh/")
			Expect(err).ToNot(HaveOccurred())
		})

		It("sets up the outgoing nats firewall for ipv6 using nftables", func() {
			format.MaxLength = 0

			// Wait for the agent to start and set up firewall rules
			Eventually(func() string {
				logs, _ := testEnvironment.RunCommand("sudo cat /var/vcap/bosh/log/current") //nolint:errcheck
				return logs
			}, 300).Should(ContainSubstring("NftablesFirewall"))

			// Check nftables for the bosh_agent table
			// nftables with inet family handles both IPv4 and IPv6
			output, err := testEnvironment.RunCommand("sudo nft list table inet bosh_agent")
			Expect(err).To(BeNil())

			// Verify table structure - inet family supports both IPv4 and IPv6
			Expect(output).To(ContainSubstring("table inet bosh_agent"))
			Expect(output).To(ContainSubstring("chain monit_access"))
			Expect(output).To(ContainSubstring("chain nats_access"))

			// The inet family in nftables automatically handles both IPv4 and IPv6
			// so we don't need separate ip6tables rules
		})
	})
})
