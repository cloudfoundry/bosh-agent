package garden_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"github.com/cloudfoundry/bosh-agent/v2/integration/utils"
)

var _ = Describe("garden container firewall", Ordered, func() {
	// Fail fast if required environment variables are missing
	BeforeAll(func() {
		Expect(utils.GardenAddress()).NotTo(BeEmpty(), "GARDEN_ADDRESS environment variable must be set")
	})

	// Run all tests against each stemcell image
	for _, stemcellImage := range utils.AllStemcellImages() {
		stemcellImage := stemcellImage // capture for closure
		imageName := utils.StemcellImageName(stemcellImage)

		Context(fmt.Sprintf("with %s", imageName), Ordered, func() {
			var gardenClient *utils.GardenClient
			var containerHandle string

			BeforeAll(func() {
				GinkgoWriter.Printf("Testing with stemcell image: %s\n", stemcellImage)

				// Create Garden client configured for this stemcell image
				var err error
				gardenClient, err = utils.NewGardenClientWithImage(stemcellImage)
				Expect(err).NotTo(HaveOccurred(), "Failed to connect to Garden at %s", utils.GardenAddress())

				// Generate unique container handle including stemcell name
				containerHandle = fmt.Sprintf("firewall-test-%s-%d", imageName, time.Now().UnixNano())
			})

			AfterAll(func() {
				if gardenClient != nil {
					// Clean up any test containers
					if err := gardenClient.Cleanup(); err != nil {
						GinkgoWriter.Printf("Warning: failed to cleanup container: %v\n", err)
					}
				}
			})

			Context("cgroup detection in container", func() {
				BeforeEach(func() {
					Expect(gardenClient).NotTo(BeNil(), "Garden client must be initialized")

					// Create fresh container for each test
					containerHandle = fmt.Sprintf("firewall-cgroup-%s-%d", imageName, time.Now().UnixNano())
					err := gardenClient.CreateStemcellContainer(containerHandle)
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					if gardenClient != nil {
						_ = gardenClient.Cleanup()
					}
				})

				It("detects cgroup version correctly inside container", func() {
					format.MaxLength = 0

					version, err := gardenClient.GetCgroupVersion()
					Expect(err).NotTo(HaveOccurred())

					GinkgoWriter.Printf("Detected cgroup version inside container: %s\n", version)

					// The exact version depends on the host, so we just verify detection works
					Expect(version).To(BeElementOf("v1", "v2", "hybrid"))
				})

				It("has nftables kernel support", func() {
					// Install nft-dump utility to check kernel support
					err := gardenClient.InstallNftDump()
					Expect(err).NotTo(HaveOccurred(), "Failed to install nft-dump utility")

					// Check kernel support using nft-dump (not nft CLI)
					available, err := gardenClient.CheckNftablesKernelSupport()
					Expect(err).NotTo(HaveOccurred())
					Expect(available).To(BeTrue(), "nftables kernel support should be available")
				})

				It("can list nftables tables using nft-dump", func() {
					// Install nft-dump utility
					err := gardenClient.InstallNftDump()
					Expect(err).NotTo(HaveOccurred(), "Failed to install nft-dump utility")

					// Try to list tables - should work in privileged container
					output, err := gardenClient.NftDumpTables()
					Expect(err).NotTo(HaveOccurred())
					GinkgoWriter.Printf("nft-dump tables output:\n%s\n", output)

					// Output should be valid YAML with a tables key
					Expect(output).To(ContainSubstring("tables:"))
				})
			})

			Context("nftables firewall rules in container", Ordered, func() {
				BeforeAll(func() {
					Expect(gardenClient).NotTo(BeNil(), "Garden client must be initialized")

					// Create container
					containerHandle = fmt.Sprintf("firewall-agent-%s-%d", imageName, time.Now().UnixNano())
					err := gardenClient.CreateStemcellContainer(containerHandle)
					Expect(err).NotTo(HaveOccurred())

					// Prepare agent environment
					err = gardenClient.PrepareAgentEnvironment()
					Expect(err).NotTo(HaveOccurred())

					// --- Install nft-dump utility for verification ---
					GinkgoWriter.Printf("Installing nft-dump utility...\n")
					err = gardenClient.InstallNftDump()
					Expect(err).NotTo(HaveOccurred(), "Failed to install nft-dump utility")

					// Verify nftables kernel support
					available, err := gardenClient.CheckNftablesKernelSupport()
					Expect(err).NotTo(HaveOccurred())
					if !available {
						Skip(fmt.Sprintf("nftables kernel support not available in %s", imageName))
					}

					// --- Copy bosh-agent binary into container ---
					// The agent binary should be pre-built at the repo root as bosh-agent-linux-amd64
					agentBinaryPath := "bosh-agent-linux-amd64"
					paths := []string{
						agentBinaryPath,
						"../../bosh-agent-linux-amd64",
					}

					var foundPath string
					for _, p := range paths {
						if _, err := os.Stat(p); err == nil {
							foundPath = p
							break
						}
					}
					Expect(foundPath).NotTo(BeEmpty(), "bosh-agent-linux-amd64 binary not found in %v - run 'go build -o bosh-agent-linux-amd64 ./main' first", paths)

					err = gardenClient.StreamIn(foundPath, "/var/vcap/bosh/bin/")
					Expect(err).NotTo(HaveOccurred())

					// Rename and make executable
					stdout, stderr, exitCode, err := gardenClient.RunCommand("sh", "-c",
						"mv /var/vcap/bosh/bin/bosh-agent-linux-amd64 /var/vcap/bosh/bin/bosh-agent && chmod +x /var/vcap/bosh/bin/bosh-agent")
					Expect(err).NotTo(HaveOccurred())
					Expect(exitCode).To(Equal(0), "Failed to setup agent binary. stdout: %s, stderr: %s", stdout, stderr)

					// Verify agent binary is in place
					stdout, stderr, exitCode, err = gardenClient.RunCommand("/var/vcap/bosh/bin/bosh-agent", "-v")
					Expect(err).NotTo(HaveOccurred())
					Expect(exitCode).To(Equal(0), "Agent version check failed. stdout: %s, stderr: %s", stdout, stderr)
					GinkgoWriter.Printf("Agent version: %s\n", strings.TrimSpace(stdout))

					// --- Create agent configuration files ---
					// agent.json tells the agent to load settings from a file
					agentConfig := map[string]interface{}{
						"Infrastructure": map[string]interface{}{
							"Settings": map[string]interface{}{
								"Sources": []map[string]interface{}{
									{
										"Type":         "File",
										"SettingsPath": "/var/vcap/bosh/settings.json",
									},
								},
							},
						},
						"Platform": map[string]interface{}{
							"Linux": map[string]interface{}{
								"EnableNATSFirewall": true,
							},
						},
					}

					agentJSON, err := json.MarshalIndent(agentConfig, "", "  ")
					Expect(err).NotTo(HaveOccurred())

					err = gardenClient.StreamInContent(agentJSON, "agent.json", "/var/vcap/bosh/", 0644)
					Expect(err).NotTo(HaveOccurred())

					// settings.json with the actual settings
					settings := map[string]interface{}{
						"agent_id": "test-agent-in-container",
						"mbus":     "https://mbus:mbus@127.0.0.1:6868",
						"ntp":      []string{},
						"blobstore": map[string]interface{}{
							"provider": "local",
							"options": map[string]interface{}{
								"blobstore_path": "/var/vcap/data/blobs",
							},
						},
						"networks": map[string]interface{}{
							"default": map[string]interface{}{
								"type":    "dynamic",
								"default": []string{"dns", "gateway"},
							},
						},
						"disks": map[string]interface{}{
							"system":     "/dev/sda",
							"persistent": map[string]interface{}{},
						},
						"vm": map[string]interface{}{
							"name": "test-vm-in-container",
						},
						"env": map[string]interface{}{
							"bosh": map[string]interface{}{
								"mbus": map[string]interface{}{
									"urls": []string{"https://mbus:mbus@127.0.0.1:6868"},
								},
							},
						},
					}

					settingsJSON, err := json.MarshalIndent(settings, "", "  ")
					Expect(err).NotTo(HaveOccurred())

					err = gardenClient.StreamInContent(settingsJSON, "settings.json", "/var/vcap/bosh/", 0644)
					Expect(err).NotTo(HaveOccurred())

					// Create a dummy bosh-agent-rc script (required by bootstrap)
					err = gardenClient.StreamInContent([]byte("#!/bin/bash\nexit 0\n"), "bosh-agent-rc", "/usr/local/bin/", 0755)
					Expect(err).NotTo(HaveOccurred())

					_, _, exitCode, err = gardenClient.RunCommand("chmod", "+x", "/usr/local/bin/bosh-agent-rc")
					Expect(err).NotTo(HaveOccurred())
					Expect(exitCode).To(Equal(0))

					// --- Start agent to create firewall rules ---
					// The agent uses the Go nftables library directly, NOT the nft CLI.
					// We use nft-dump to verify rules were created.
					// Note: PrepareAgentEnvironment already unmounts Garden's bind-mounted files
					// (/etc/resolv.conf, /etc/hosts, /etc/hostname) to prevent "Device or resource busy" errors.
					GinkgoWriter.Printf("Starting bosh-agent to create firewall rules...\n")
					stdout, stderr, exitCode, err = gardenClient.RunCommandWithTimeout(30*time.Second, "sh", "-c", `
# Start agent in background with the config file
/var/vcap/bosh/bin/bosh-agent -P ubuntu -C /var/vcap/bosh/agent.json &
AGENT_PID=$!

# Wait for firewall rules to be created
# Use nft-dump to check (it uses the Go nftables library, not CLI)
for i in $(seq 1 20); do
	sleep 1
	if /var/vcap/bosh/bin/nft-dump table inet bosh_agent 2>/dev/null | grep -q "monit_access"; then
		echo "Firewall rules created after ${i}s (verified via nft-dump)"
		break
	fi
	# After 15 seconds, assume rules were created
	if [ $i -ge 15 ]; then
		echo "Assuming firewall rules created after ${i}s (timeout)"
		break
	fi
done

# Kill the agent
kill $AGENT_PID 2>/dev/null || true
sleep 1

echo "Agent startup completed"
`)

					// Don't fail on timeout - agent might not start cleanly without proper env
					if err != nil && !strings.Contains(err.Error(), "timed out") {
						Fail(fmt.Sprintf("Agent startup failed: %v, stdout: %s, stderr: %s", err, stdout, stderr))
					}

					GinkgoWriter.Printf("Agent startup output:\nstdout: %s\nstderr: %s\nexit: %d\n", stdout, stderr, exitCode)
				})

				AfterAll(func() {
					if gardenClient != nil {
						_ = gardenClient.Cleanup()
					}
				})

				It("agent created the bosh_agent firewall table", func() {
					// Verify firewall rules were created by the agent using nft-dump
					ruleOutput, err := gardenClient.NftDumpTable("inet", "bosh_agent")
					Expect(err).NotTo(HaveOccurred(), "Agent failed to create firewall table")

					GinkgoWriter.Printf("nftables rules (YAML):\n%s\n", ruleOutput)

					// Verify table info
					Expect(ruleOutput).To(ContainSubstring("family: inet"))
					Expect(ruleOutput).To(ContainSubstring("name: bosh_agent"))

					// Verify monit_access chain exists
					Expect(ruleOutput).To(ContainSubstring("name: monit_access"))
				})

				It("creates firewall rules with appropriate socket matching", func() {
					// Get the nftables rules using nft-dump
					ruleOutput, err := gardenClient.NftDumpTable("inet", "bosh_agent")
					Expect(err).NotTo(HaveOccurred(), "bosh_agent table not found")

					// Log the cgroup version for debugging
					cgroupVersion, _ := gardenClient.GetCgroupVersion()
					GinkgoWriter.Printf("Cgroup version: %s\n", cgroupVersion)
					GinkgoWriter.Printf("nftables rules (YAML):\n%s\n", ruleOutput)

					// Verify firewall rules are present with the expected structure.
					// NOTE: The Go nftables library doesn't support unmarshaling socket expressions
					// (socket cgroupv2), so we can't directly verify cgroup matching. However, we can
					// verify the rule structure and mark setting which indicates the rule is working.
					//
					// The nft-dump output shows human-readable match types when supported:
					// - cgroup v2: "cgroupv2 cgroup_id=XXX" in match field (if library supported it)
					// - cgroup v1: "cgroup classid=0xXXX" in match field
					// - fallback: "skuid uid=XXX" in match field (UID-based)
					//
					// Since cgroup v2 socket matching isn't readable, we verify:
					// 1. The monit_access chain exists with rules
					// 2. Rules match the expected destination (127.0.0.1 port 2822 for monit)
					// 3. Rules set the bosh marker (0xb054) and accept

					// Verify the expected chains exist
					Expect(ruleOutput).To(ContainSubstring("name: monit_access"))
					Expect(ruleOutput).To(ContainSubstring("name: nats_access"))

					// Verify monit rules are present with correct destination
					Expect(ruleOutput).To(ContainSubstring("dport 2822"), "monit port should be in rules")
					Expect(ruleOutput).To(ContainSubstring("daddr 127.0.0.1"), "monit address should be in rules")

					// Verify the bosh marker is being set (indicates the rule is complete)
					Expect(ruleOutput).To(ContainSubstring("mark set 0xb054"), "bosh marker should be set")

					// Verify accept action
					Expect(ruleOutput).To(ContainSubstring("accept"), "rules should accept matching traffic")
				})
			})
		})
	}
})
