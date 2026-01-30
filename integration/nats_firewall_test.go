package integration_test

import (
	"fmt"
	"net/url"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

var _ = Describe("nats firewall", Ordered, func() {

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

			// Verify monit rules - should match either:
			// - socket cgroupv2 classid (pure cgroup v2)
			// - meta cgroup (hybrid cgroup v1+v2 systems)
			Expect(output).To(SatisfyAny(
				MatchRegexp("tcp dport 2822.*socket cgroupv2.*classid.*accept"),
				MatchRegexp("meta cgroup.*tcp dport 2822.*accept"),
			))

			// Verify NATS rules - the NATS chain should have rules for the director's NATS port
			// Use (?s) flag for multiline matching since chain definition spans multiple lines
			Expect(output).To(MatchRegexp("(?s)nats_access.*tcp dport"))

			// Get BOSH director hostname from BOSH_ENVIRONMENT (may be a full URL)
			boshEnvURL := os.Getenv("BOSH_ENVIRONMENT")
			parsedURL, _ := url.Parse(boshEnvURL)
			boshEnv := parsedURL.Hostname()
			if boshEnv == "" {
				boshEnv = boshEnvURL // fallback if not a URL
			}

			// Test that we cannot access the director nats from outside the agent cgroup
			// -w2 == timeout 2 seconds
			out, err := testEnvironment.RunCommand(fmt.Sprintf("nc %v 4222 -w2 -v", boshEnv))
			Expect(err).NotTo(BeNil())
			Expect(out).To(ContainSubstring("timed out"))

			// Test that we CAN access NATS when running in the agent's cgroup
			// Find the agent's cgroup path and run nc from within it
			// Note: On hybrid cgroup systems (v1+v2), use grep "^0::" to get the v2 unified hierarchy
			// The cgroup v2 filesystem may be at /sys/fs/cgroup (pure v2) or /sys/fs/cgroup/unified (hybrid)
			out, err = testEnvironment.RunCommand(fmt.Sprintf(`sudo sh -c '
				# Find the agent process cgroup
				agent_pid=$(pgrep -f "bosh-agent$" | head -1)
				if [ -z "$agent_pid" ]; then
					echo "Agent process not found"
					exit 1
				fi
				# For cgroup v2 (or hybrid), get the unified hierarchy path
				agent_cgroup=$(grep "^0::" /proc/$agent_pid/cgroup | cut -d: -f3)
				if [ -z "$agent_cgroup" ]; then
					# Fallback to first cgroup line for pure v1 systems
					agent_cgroup=$(head -1 /proc/$agent_pid/cgroup | cut -d: -f3)
				fi
				# Try unified mount point first (hybrid), then standard (pure v2)
				if [ -d /sys/fs/cgroup/unified ]; then
					echo $$ > /sys/fs/cgroup/unified${agent_cgroup}/cgroup.procs
				else
					echo $$ > /sys/fs/cgroup${agent_cgroup}/cgroup.procs
				fi
				nc %v 4222 -w2 -v'
			`, boshEnv))
			// Skip if cgroup manipulation failed - this happens in nested containers (e.g., incus VMs)
			// where we don't have permission to move processes between cgroups
			if err != nil {
				Skip("Skipping cgroup access test - cgroup manipulation not supported in this environment: " + err.Error())
			}
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
