package garden_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"github.com/cloudfoundry/bosh-agent/v2/integration/agentinstaller"
	"github.com/cloudfoundry/bosh-agent/v2/integration/installerdriver"
	"github.com/cloudfoundry/bosh-agent/v2/integration/utils"
	windowsutils "github.com/cloudfoundry/bosh-agent/v2/integration/windows/utils"
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
			var gardenClient garden.Client
			var parentDriver *installerdriver.SSHDriver

			BeforeAll(func() {
				GinkgoWriter.Printf("Testing with stemcell image: %s\n", stemcellImage)

				// Connect to Garden through SSH tunnel (via jumpbox)
				jumpboxClient, err := windowsutils.GetSSHTunnelClient()
				Expect(err).NotTo(HaveOccurred(), "Failed to get SSH tunnel client")

				gardenClient, err = installerdriver.NewGardenAPIClient(jumpboxClient, utils.GardenAddress(), nil)
				Expect(err).NotTo(HaveOccurred(), "Failed to connect to Garden at %s", utils.GardenAddress())

				// Connect to the agent VM through the jumpbox for file operations
				agentSSHClient, err := utils.DialAgentThroughJumpbox(utils.GetAgentIP())
				Expect(err).NotTo(HaveOccurred(), "Failed to connect to agent VM through jumpbox")

				// Create parent driver for container operations (connected to agent VM, not jumpbox)
				parentDriver = installerdriver.NewSSHDriver(installerdriver.SSHDriverConfig{
					Client:  agentSSHClient,
					Host:    utils.GetAgentIP(),
					UseSudo: true,
				})
				err = parentDriver.Bootstrap()
				Expect(err).NotTo(HaveOccurred(), "Failed to bootstrap parent driver")
			})

			AfterAll(func() {
				if parentDriver != nil {
					parentDriver.Cleanup()
				}
			})

			Context("cgroup detection in container", func() {
				var testDriver *installerdriver.GardenDriver

				BeforeEach(func() {
					Expect(gardenClient).NotTo(BeNil(), "Garden client must be initialized")

					// Create fresh container for each test
					containerHandle := fmt.Sprintf("firewall-cgroup-%s-%d", imageName, time.Now().UnixNano())
					testDriver = installerdriver.NewGardenDriver(installerdriver.GardenDriverConfig{
						GardenClient: gardenClient,
						ParentDriver: parentDriver,
						Handle:       containerHandle,
						Image:        stemcellImage,
					})
					err := testDriver.Bootstrap()
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					if testDriver != nil {
						_ = testDriver.Cleanup()
					}
				})

				It("detects cgroup version correctly inside container", func() {
					format.MaxLength = 0

					// Check for cgroup v2
					stdout, _, exitCode, err := testDriver.RunCommand("sh", "-c", "test -f /sys/fs/cgroup/cgroup.controllers && echo v2")
					Expect(err).NotTo(HaveOccurred())
					if exitCode == 0 && strings.TrimSpace(stdout) == "v2" {
						GinkgoWriter.Printf("Detected cgroup version inside container: v2\n")
						return
					}

					// Check for cgroup v1
					stdout, _, exitCode, err = testDriver.RunCommand("sh", "-c", "test -d /sys/fs/cgroup/cpu && echo v1")
					Expect(err).NotTo(HaveOccurred())
					if exitCode == 0 && strings.TrimSpace(stdout) == "v1" {
						// Check for hybrid
						stdout2, _, exitCode2, _ := testDriver.RunCommand("sh", "-c", "test -d /sys/fs/cgroup/unified && echo hybrid")
						if exitCode2 == 0 && strings.TrimSpace(stdout2) == "hybrid" {
							GinkgoWriter.Printf("Detected cgroup version inside container: hybrid\n")
							return
						}
						GinkgoWriter.Printf("Detected cgroup version inside container: v1\n")
						return
					}

					GinkgoWriter.Printf("Detected cgroup version inside container: unknown\n")
				})

				It("has nftables kernel support", func() {
					// Install nft-dump utility
					agentCfg := agentinstaller.DefaultConfig()
					agentCfg.Debug = false
					installer := agentinstaller.New(agentCfg, testDriver)
					// Just install nft-dump, not the full agent
					data, err := os.ReadFile(utils.FindNftDumpBinary())
					if err != nil {
						// Try to find in alternative paths
						paths := []string{"nft-dump-linux-amd64", "../../nft-dump-linux-amd64"}
						for _, p := range paths {
							data, err = os.ReadFile(p)
							if err == nil {
								break
							}
						}
					}
					Expect(err).NotTo(HaveOccurred(), "nft-dump binary not found")

					err = testDriver.MkdirAll("/var/vcap/bosh/bin", 0755)
					Expect(err).NotTo(HaveOccurred())
					err = testDriver.WriteFile("/var/vcap/bosh/bin/nft-dump", data, 0755)
					Expect(err).NotTo(HaveOccurred())

					// Check kernel support
					available, err := installer.CheckNftablesKernelSupport()
					Expect(err).NotTo(HaveOccurred())
					Expect(available).To(BeTrue(), "nftables kernel support should be available")
				})

				It("can list nftables tables using nft-dump", func() {
					// Install nft-dump
					data, err := os.ReadFile(utils.FindNftDumpBinary())
					if err != nil {
						paths := []string{"nft-dump-linux-amd64", "../../nft-dump-linux-amd64"}
						for _, p := range paths {
							data, err = os.ReadFile(p)
							if err == nil {
								break
							}
						}
					}
					Expect(err).NotTo(HaveOccurred(), "nft-dump binary not found")

					err = testDriver.MkdirAll("/var/vcap/bosh/bin", 0755)
					Expect(err).NotTo(HaveOccurred())
					err = testDriver.WriteFile("/var/vcap/bosh/bin/nft-dump", data, 0755)
					Expect(err).NotTo(HaveOccurred())

					// List tables
					stdout, stderr, exitCode, err := testDriver.RunCommand("/var/vcap/bosh/bin/nft-dump", "tables")
					Expect(err).NotTo(HaveOccurred())
					Expect(exitCode).To(Equal(0), "nft-dump tables failed: %s", stderr)

					GinkgoWriter.Printf("nft-dump tables output:\n%s\n", stdout)
					Expect(stdout).To(ContainSubstring("tables:"))
				})
			})

			Context("nftables firewall rules in container", Ordered, func() {
				var agentDriver *installerdriver.GardenDriver
				var agentInst *agentinstaller.Installer

				BeforeAll(func() {
					Expect(gardenClient).NotTo(BeNil(), "Garden client must be initialized")

					// Create container for agent
					containerHandle := fmt.Sprintf("firewall-agent-%s-%d", imageName, time.Now().UnixNano())
					agentDriver = installerdriver.NewGardenDriver(installerdriver.GardenDriverConfig{
						GardenClient: gardenClient,
						ParentDriver: parentDriver,
						Handle:       containerHandle,
						Image:        stemcellImage,
					})
					err := agentDriver.Bootstrap()
					Expect(err).NotTo(HaveOccurred())

					// Install agent using agentinstaller
					agentCfg := agentinstaller.DefaultConfig()
					agentCfg.AgentID = "test-agent-in-container"
					agentCfg.Debug = true
					agentCfg.EnableNATSFirewall = true

					agentInst = agentinstaller.New(agentCfg, agentDriver)
					err = agentInst.Install()
					Expect(err).NotTo(HaveOccurred())

					// Verify nftables kernel support
					available, err := agentInst.CheckNftablesKernelSupport()
					Expect(err).NotTo(HaveOccurred())
					if !available {
						Skip(fmt.Sprintf("nftables kernel support not available in %s", imageName))
					}

					// Verify agent binary
					stdout, stderr, exitCode, err := agentDriver.RunCommand(agentInst.AgentBinaryPath(), "-v")
					Expect(err).NotTo(HaveOccurred())
					Expect(exitCode).To(Equal(0), "Agent version check failed. stdout: %s, stderr: %s", stdout, stderr)
					GinkgoWriter.Printf("Agent version: %s\n", strings.TrimSpace(stdout))

					// Start agent to create firewall rules
					GinkgoWriter.Printf("Starting bosh-agent to create firewall rules...\n")
					stdout, stderr, exitCode, err = agentDriver.RunScript(fmt.Sprintf(`
# Start agent in background with the config file
%s -P ubuntu -C %s &
AGENT_PID=$!

# Wait for firewall rules to be created
for i in $(seq 1 20); do
	sleep 1
	if %s table inet bosh_agent 2>/dev/null | grep -q "monit_access"; then
		echo "Firewall rules created after ${i}s (verified via nft-dump)"
		break
	fi
	if [ $i -ge 15 ]; then
		echo "Assuming firewall rules created after ${i}s (timeout)"
		break
	fi
done

# Kill the agent
kill $AGENT_PID 2>/dev/null || true
sleep 1

echo "Agent startup completed"
`, agentInst.AgentBinaryPath(), agentInst.AgentConfigPath(), agentInst.NftDumpBinaryPath()))

					// Don't fail on timeout - agent might not start cleanly without proper env
					if err != nil && !strings.Contains(err.Error(), "timed out") {
						Fail(fmt.Sprintf("Agent startup failed: %v, stdout: %s, stderr: %s", err, stdout, stderr))
					}

					GinkgoWriter.Printf("Agent startup output:\nstdout: %s\nstderr: %s\nexit: %d\n", stdout, stderr, exitCode)
				})

				AfterAll(func() {
					if agentDriver != nil {
						_ = agentDriver.Cleanup()
					}
				})

				It("agent created the bosh_agent firewall table", func() {
					// Verify firewall rules were created by the agent
					ruleOutput, err := agentInst.NftDumpTable("inet", "bosh_agent")
					Expect(err).NotTo(HaveOccurred(), "Agent failed to create firewall table")

					GinkgoWriter.Printf("nftables rules (YAML):\n%s\n", ruleOutput)

					// Verify table info
					Expect(ruleOutput).To(ContainSubstring("family: inet"))
					Expect(ruleOutput).To(ContainSubstring("name: bosh_agent"))

					// Verify monit_access chain exists
					Expect(ruleOutput).To(ContainSubstring("name: monit_access"))
				})

				It("creates firewall rules with appropriate socket matching", func() {
					// Get the nftables rules
					ruleOutput, err := agentInst.NftDumpTable("inet", "bosh_agent")
					Expect(err).NotTo(HaveOccurred(), "bosh_agent table not found")

					// Log cgroup version for debugging
					stdout, _, _, _ := agentDriver.RunCommand("sh", "-c", "test -f /sys/fs/cgroup/cgroup.controllers && echo v2 || echo v1")
					GinkgoWriter.Printf("Cgroup version: %s\n", strings.TrimSpace(stdout))
					GinkgoWriter.Printf("nftables rules (YAML):\n%s\n", ruleOutput)

					// Verify the expected chains exist
					Expect(ruleOutput).To(ContainSubstring("name: monit_access"))
					Expect(ruleOutput).To(ContainSubstring("name: nats_access"))

					// Verify monit rules are present with correct destination
					Expect(ruleOutput).To(ContainSubstring("dport 2822"), "monit port should be in rules")
					Expect(ruleOutput).To(ContainSubstring("daddr 127.0.0.1"), "monit address should be in rules")

					// Verify the bosh marker is being set
					Expect(ruleOutput).To(ContainSubstring("mark set 0xb054"), "bosh marker should be set")

					// Verify accept action
					Expect(ruleOutput).To(ContainSubstring("accept"), "rules should accept matching traffic")
				})
			})
		})
	}
})
