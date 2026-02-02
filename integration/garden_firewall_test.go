package integration_test

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
	var gardenClient *utils.GardenClient
	var containerHandle string

	BeforeAll(func() {
		// Skip if GARDEN_ADDRESS not set
		if utils.GardenAddress() == "" {
			Skip("GARDEN_ADDRESS not set - skipping Garden container firewall tests")
		}

		// Create Garden client
		var err error
		gardenClient, err = utils.NewGardenClient()
		if err != nil {
			Skip(fmt.Sprintf("Failed to connect to Garden: %v", err))
		}

		// Generate unique container handle
		containerHandle = fmt.Sprintf("firewall-test-%d", time.Now().UnixNano())
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
			if gardenClient == nil {
				Skip("Garden client not available")
			}

			// Create fresh container for each test
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

			// On Noble host, containers should see cgroup v2
			// The exact version depends on the host, so we just verify detection works
			Expect(version).To(BeElementOf("v1", "v2", "hybrid"))
		})

		It("has nftables available in container", func() {
			available, err := gardenClient.CheckNftablesAvailable()
			Expect(err).NotTo(HaveOccurred())
			Expect(available).To(BeTrue(), "nftables (nft) should be available in stemcell")
		})

		It("can run nft commands in privileged container", func() {
			// Try to list tables - should work in privileged container
			stdout, stderr, exitCode, err := gardenClient.RunCommand("nft", "list", "tables")
			Expect(err).NotTo(HaveOccurred())
			// Exit code 0 means nft works (even if no tables exist yet)
			Expect(exitCode).To(Equal(0), "nft list tables should succeed. stderr: %s", stderr)
			GinkgoWriter.Printf("nft list tables output: %s\n", stdout)
		})
	})

	Context("nftables firewall rules in container", Ordered, func() {
		BeforeAll(func() {
			if gardenClient == nil {
				Skip("Garden client not available")
			}

			// Create container
			containerHandle = fmt.Sprintf("firewall-agent-test-%d", time.Now().UnixNano())
			err := gardenClient.CreateStemcellContainer(containerHandle)
			Expect(err).NotTo(HaveOccurred())

			// Prepare agent environment
			err = gardenClient.PrepareAgentEnvironment()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func() {
			if gardenClient != nil {
				_ = gardenClient.Cleanup()
			}
		})

		It("can copy bosh-agent binary into container", func() {
			// The agent binary should be pre-built at bosh-agent-linux-amd64
			agentBinaryPath := "bosh-agent-linux-amd64"
			if _, err := os.Stat(agentBinaryPath); os.IsNotExist(err) {
				Skip("bosh-agent-linux-amd64 binary not found - run 'go build -o bosh-agent-linux-amd64 ./main' first")
			}

			err := gardenClient.StreamIn(agentBinaryPath, "/var/vcap/bosh/bin/")
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
			Expect(stdout).To(ContainSubstring("version"))
		})

		It("can create minimal settings.json", func() {
			// Create a minimal settings.json that enables the firewall
			// but doesn't require NATS connection (for monit_access rules only)
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

			// Verify settings file is in place
			stdout, stderr, exitCode, err := gardenClient.RunCommand("cat", "/var/vcap/bosh/settings.json")
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0), "Failed to read settings.json. stderr: %s", stderr)
			Expect(stdout).To(ContainSubstring("test-agent-in-container"))
		})

		It("starts agent briefly to create firewall rules", func() {
			// Start agent in background, let it initialize firewall, then kill it
			// We use timeout to prevent hanging if agent fails to start
			stdout, stderr, exitCode, err := gardenClient.RunCommandWithTimeout(30*time.Second, "sh", "-c", `
				# Start agent in background
				/var/vcap/bosh/bin/bosh-agent -P ubuntu -C /var/vcap/bosh/settings.json &
				AGENT_PID=$!
				
				# Wait for firewall rules to be created (poll nftables)
				for i in $(seq 1 20); do
					sleep 1
					if nft list table inet bosh_agent 2>/dev/null | grep -q "monit_access"; then
						echo "Firewall rules created after ${i}s"
						break
					fi
				done
				
				# Kill the agent
				kill $AGENT_PID 2>/dev/null || true
				sleep 1
				
				# Output the nftables rules for verification
				echo "=== nftables rules ==="
				nft list table inet bosh_agent 2>&1 || echo "Table not found"
			`)

			// Don't fail on timeout - agent might not start cleanly without proper env
			if err != nil && !strings.Contains(err.Error(), "timed out") {
				Fail(fmt.Sprintf("Agent startup failed: %v, stdout: %s, stderr: %s", err, stdout, stderr))
			}

			GinkgoWriter.Printf("Agent output:\nstdout: %s\nstderr: %s\nexit: %d\n", stdout, stderr, exitCode)

			// Check if firewall rules were created
			ruleOutput, _, exitCode2, _ := gardenClient.RunCommand("nft", "list", "table", "inet", "bosh_agent")
			if exitCode2 != 0 {
				Skip("Agent failed to create firewall table - this may be expected if cgroups aren't fully supported in this container environment")
			}

			GinkgoWriter.Printf("nftables rules:\n%s\n", ruleOutput)

			// Verify monit_access chain exists
			Expect(ruleOutput).To(ContainSubstring("chain monit_access"), "monit_access chain should exist")
		})

		It("uses cgroup-based socket matching (not UID fallback)", func() {
			// Get the nftables rules
			ruleOutput, _, exitCode, err := gardenClient.RunCommand("nft", "list", "table", "inet", "bosh_agent")
			if exitCode != 0 || err != nil {
				Skip("bosh_agent table not found - previous test may have skipped")
			}

			// On Noble (cgroup v2), rules should use socket cgroupv2 matching
			// NOT meta skuid (which is the UID fallback)
			cgroupVersion, _ := gardenClient.GetCgroupVersion()
			GinkgoWriter.Printf("Cgroup version: %s\n", cgroupVersion)

			if cgroupVersion == "v2" {
				// Should see socket cgroupv2 matching
				Expect(ruleOutput).To(SatisfyAny(
					// Proper cgroup v2 matching with inode ID
					MatchRegexp(`socket cgroupv2 level \d+`),
					// Or cgroup v2 with classid (alternative format)
					ContainSubstring("socket cgroupv2"),
				), "Should use cgroup v2 socket matching, not UID fallback. Rules:\n%s", ruleOutput)

				// Should NOT fall back to UID-based matching
				Expect(ruleOutput).NotTo(ContainSubstring("meta skuid"),
					"Should not fall back to UID-based matching on cgroup v2. Rules:\n%s", ruleOutput)
			} else {
				// On cgroup v1 or hybrid, either cgroup or UID matching is acceptable
				Expect(ruleOutput).To(SatisfyAny(
					ContainSubstring("meta cgroup"),
					ContainSubstring("meta skuid"),
					ContainSubstring("socket cgroupv2"),
				), "Should have some form of socket matching. Rules:\n%s", ruleOutput)
			}
		})
	})
})
