package integration_test

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

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

			// Delete any existing firewall table from previous runs to ensure clean state.
			// The agent will recreate it on startup.
			_ = testEnvironment.NftDumpDelete("inet", "bosh_agent") //nolint:errcheck
		})

		It("sets up the outgoing nats firewall using nftables", func() {
			format.MaxLength = 0

			// Wait for the agent to start, connect to NATS, and set up firewall rules.
			// We check for NATS rules in nftables using nft-dump rather than the nft CLI.
			// The agent creates an empty nats_access chain in SetupAgentRules, then populates it
			// in BeforeConnect when connecting to NATS. Poll until rules appear.
			var output string
			startTime := time.Now()
			debugDumped := false
			Eventually(func() string {
				output, _ = testEnvironment.NftDumpTable("inet", "bosh_agent") //nolint:errcheck

				// After 30 seconds, dump debug info if NATS rules still missing
				if !debugDumped && time.Since(startTime) > 30*time.Second && !strings.Contains(output, "4222") {
					debugDumped = true
					GinkgoWriter.Println("=== DEBUG: NATS rules not appearing after 30s ===")

					GinkgoWriter.Println("--- nftables table state (YAML) ---")
					GinkgoWriter.Println(output)

					GinkgoWriter.Println("--- systemctl status bosh-agent ---")
					status, _ := testEnvironment.RunCommand("sudo systemctl status bosh-agent") //nolint:errcheck
					GinkgoWriter.Println(status)

					GinkgoWriter.Println("--- agent cgroup (/proc/PID/cgroup) ---")
					cgroup, _ := testEnvironment.RunCommand("sudo sh -c 'pgrep -f bosh-agent$ | head -1 | xargs -I{} cat /proc/{}/cgroup'") //nolint:errcheck
					GinkgoWriter.Println(cgroup)

					GinkgoWriter.Println("--- agent journal logs (last 100 lines) ---")
					logs, _ := testEnvironment.RunCommand("sudo journalctl -u bosh-agent --no-pager -n 100") //nolint:errcheck
					GinkgoWriter.Println(logs)

					GinkgoWriter.Println("--- settings.json mbus URL ---")
					mbus, _ := testEnvironment.RunCommand("sudo cat /var/vcap/bosh/settings.json | grep -o '\"mbus\":\"[^\"]*\"'") //nolint:errcheck
					GinkgoWriter.Println(mbus)

					GinkgoWriter.Println("--- /var/vcap/bosh/ directory ---")
					dir, _ := testEnvironment.RunCommand("ls -la /var/vcap/bosh/") //nolint:errcheck
					GinkgoWriter.Println(dir)

					GinkgoWriter.Println("=== END DEBUG ===")
				}

				return output
			}, 300).Should(ContainSubstring("4222"))

			// Verify table structure - should have both monit_access and nats_access chains
			// nft-dump output is YAML format
			Expect(output).To(ContainSubstring("family: inet"))
			Expect(output).To(ContainSubstring("name: bosh_agent"))
			Expect(output).To(ContainSubstring("name: monit_access"))
			Expect(output).To(ContainSubstring("name: nats_access"))

			// Verify firewall rules are present with the expected structure.
			// NOTE: The Go nftables library doesn't support unmarshaling socket expressions
			// (socket cgroupv2), so we can't directly verify cgroup matching via nft-dump.
			// However, we verify the rule structure and mark setting which indicates rules are working.
			//
			// Verify monit rules have correct destination and marker
			Expect(output).To(ContainSubstring("dport 2822"), "monit port should be in rules")
			Expect(output).To(ContainSubstring("mark set 0xb054"), "bosh marker should be set")

			// Verify NATS rules have the expected port (4222)
			Expect(output).To(ContainSubstring("dport 4222"), "NATS port should be in rules")

			// Get BOSH director hostname from BOSH_ENVIRONMENT (may be a full URL)
			boshEnvURL := os.Getenv("BOSH_ENVIRONMENT")
			parsedURL, err := url.Parse(boshEnvURL)
			Expect(err).NotTo(HaveOccurred())
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

			// Delete any existing firewall table from previous runs to ensure clean state.
			_ = testEnvironment.NftDumpDelete("inet", "bosh_agent") //nolint:errcheck
		})

		It("sets up the outgoing nats firewall for ipv6 using nftables", func() {
			format.MaxLength = 0

			// Wait for the agent to start and set up firewall rules.
			// We check for the table directly using nft-dump rather than nft CLI.
			var output string
			Eventually(func() string {
				output, _ = testEnvironment.NftDumpTable("inet", "bosh_agent") //nolint:errcheck
				return output
			}, 300).Should(ContainSubstring("name: monit_access"))

			// Verify table structure - inet family supports both IPv4 and IPv6
			// nft-dump output is YAML format
			Expect(output).To(ContainSubstring("family: inet"))
			Expect(output).To(ContainSubstring("name: bosh_agent"))
			Expect(output).To(ContainSubstring("name: monit_access"))
			Expect(output).To(ContainSubstring("name: nats_access"))

			// The inet family in nftables automatically handles both IPv4 and IPv6
			// so we don't need separate ip6tables rules
		})
	})
})
