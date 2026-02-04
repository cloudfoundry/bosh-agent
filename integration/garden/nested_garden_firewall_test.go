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
	"github.com/cloudfoundry/bosh-agent/v2/integration/gardeninstaller"
	"github.com/cloudfoundry/bosh-agent/v2/integration/installerdriver"
	"github.com/cloudfoundry/bosh-agent/v2/integration/utils"
	windowsutils "github.com/cloudfoundry/bosh-agent/v2/integration/windows/utils"
)

// Nested Garden test ports and network configuration
const (
	// L1 Garden listens on 7777 inside container
	L1ContainerPort uint32 = 7777

	// L2 Garden listens on 7777 inside L1 container
	L2ContainerPort uint32 = 7777
)

// Default disk limit for nested containers (40GB)
const defaultDiskLimit = uint64(40 * 1024 * 1024 * 1024)

// collectL1Diagnostics collects diagnostic information from L1 container when L2 installation fails.
func collectL1Diagnostics(l1Driver installerdriver.Driver, context string) {
	GinkgoWriter.Printf("\n========== L1 DIAGNOSTICS (%s) ==========\n", context)

	// Check if we can still communicate with L1 container
	stdout, stderr, exitCode, err := l1Driver.RunCommand("echo", "L1-health-check")
	if err != nil {
		GinkgoWriter.Printf("L1 container unreachable: err=%v\n", err)
		return
	}
	if exitCode != 0 {
		GinkgoWriter.Printf("L1 health check failed: exit=%d stdout=%s stderr=%s\n", exitCode, stdout, stderr)
		return
	}
	GinkgoWriter.Printf("L1 container reachable\n")

	// Check disk space
	stdout, stderr, exitCode, err = l1Driver.RunCommand("df", "-h")
	if err == nil && exitCode == 0 {
		GinkgoWriter.Printf("\n--- L1 Disk Space ---\n%s\n", stdout)
	} else {
		GinkgoWriter.Printf("Failed to get disk space: err=%v exit=%d stderr=%s\n", err, exitCode, stderr)
	}

	// Check memory
	stdout, stderr, exitCode, err = l1Driver.RunCommand("free", "-m")
	if err == nil && exitCode == 0 {
		GinkgoWriter.Printf("\n--- L1 Memory ---\n%s\n", stdout)
	}

	// Check if L1 Garden process is running
	stdout, _, _, err = l1Driver.RunScript("ps aux | grep -E 'garden|gdn|containerd' | grep -v grep || echo 'No garden processes found'")
	if err == nil {
		GinkgoWriter.Printf("\n--- L1 Garden Processes ---\n%s\n", stdout)
	}

	// Check L1 Garden logs (last 50 lines)
	stdout, _, _, err = l1Driver.RunScript("tail -50 /var/vcap/sys/log/garden/*.log 2>/dev/null || echo 'No garden logs found'")
	if err == nil {
		GinkgoWriter.Printf("\n--- L1 Garden Logs (last 50 lines) ---\n%s\n", stdout)
	}

	GinkgoWriter.Printf("\n========== END L1 DIAGNOSTICS ==========\n\n")
}

var _ = Describe("nested garden firewall", Ordered, func() {
	var (
		releaseTarball string
		agentIP        string
	)

	BeforeAll(func() {
		// Nested Garden tests require the compiled release tarball
		releaseTarball = utils.GetReleaseTarball()
		if releaseTarball == "" {
			Skip("GARDEN_RELEASE_TARBALL not set - skipping nested Garden tests")
		}

		// Verify the tarball exists
		if _, err := os.Stat(releaseTarball); err != nil {
			Skip("GARDEN_RELEASE_TARBALL does not exist: " + releaseTarball)
		}

		// Get agent IP for connecting to nested Garden
		agentIP = utils.GetAgentIP()
		if agentIP == "" {
			Skip("AGENT_IP not set - cannot connect to nested Garden")
		}

		// Verify SSH client for tunneling is available
		_, err := windowsutils.GetSSHTunnelClient()
		if err != nil {
			Skip("Failed to get SSH tunnel client: " + err.Error())
		}

		GinkgoWriter.Printf("Nested Garden tests using:\n")
		GinkgoWriter.Printf("  Release tarball: %s\n", releaseTarball)
		GinkgoWriter.Printf("  Agent IP: %s\n", agentIP)
	})

	// Test with Noble stemcell (primary target for nested Garden)
	Context("with ubuntu-noble-stemcell", Ordered, func() {
		var (
			// Host Garden client (L0)
			hostGardenClient garden.Client

			// L1 container - Garden running inside host Garden
			l1Driver        *installerdriver.GardenDriver
			l1GardenClient  garden.Client
			l1Installer     *gardeninstaller.Installer
			l1GardenAddress string

			// L2 container - Garden running inside L1 Garden
			l2Driver        *installerdriver.GardenDriver
			l2GardenClient  garden.Client
			l2Installer     *gardeninstaller.Installer
			l2GardenAddress string
		)

		BeforeAll(func() {
			format.MaxLength = 0

			// Connect to host Garden through SSH tunnel
			gardenAddr := utils.GardenAddress()
			Expect(gardenAddr).NotTo(BeEmpty(), "GARDEN_ADDRESS must be set")

			sshTunnelClient, err := windowsutils.GetSSHTunnelClient()
			Expect(err).NotTo(HaveOccurred(), "Failed to get SSH tunnel client")

			hostGardenClient, err = installerdriver.NewGardenAPIClient(sshTunnelClient, gardenAddr, nil)
			Expect(err).NotTo(HaveOccurred(), "Failed to connect to host Garden")

			GinkgoWriter.Printf("Connected to host Garden at %s\n", gardenAddr)
		})

		AfterAll(func() {
			// Clean up L1 (which will also clean up any L2 containers)
			if l1Installer != nil {
				GinkgoWriter.Printf("Stopping L1 Garden...\n")
				if err := l1Installer.Stop(); err != nil {
					GinkgoWriter.Printf("Warning: failed to stop L1 Garden: %v\n", err)
				}
			}

			if l1Driver != nil {
				GinkgoWriter.Printf("Cleaning up L1 container...\n")
				if err := l1Driver.Cleanup(); err != nil {
					GinkgoWriter.Printf("Warning: failed to cleanup L1 container: %v\n", err)
				}
			}
		})

		Context("Level 1: Garden inside host Garden container", Ordered, func() {
			BeforeAll(func() {
				// Create L1 container handle
				l1Handle := fmt.Sprintf("l1-garden-%d", time.Now().UnixNano())

				// Connect to the agent VM through the jumpbox for file operations.
				// This SSH connection is used by the parentDriver to create directories
				// on the agent VM (not the jumpbox).
				agentSSHClient, err := utils.DialAgentThroughJumpbox(agentIP)
				Expect(err).NotTo(HaveOccurred(), "Failed to connect to agent VM through jumpbox")

				// Create parent driver that connects to the agent VM
				parentDriver := installerdriver.NewSSHDriver(installerdriver.SSHDriverConfig{
					Client:  agentSSHClient,
					Host:    agentIP,
					UseSudo: true,
				})
				err = parentDriver.Bootstrap()
				Expect(err).NotTo(HaveOccurred(), "Failed to bootstrap parent driver")

				// Create L1 GardenDriver with config
				// Let Garden dynamically allocate an IP from its pool (10.254.0.0/22).
				// This ensures proper routing from the agent VM through SSH tunnel.
				l1Driver = installerdriver.NewGardenDriver(installerdriver.GardenDriverConfig{
					GardenClient: hostGardenClient,
					ParentDriver: parentDriver,
					Handle:       l1Handle,
					Image:        utils.NobleStemcellImage,
					// Network is empty - let Garden allocate from its pool
					DiskLimit: defaultDiskLimit,
				})

				// Bootstrap L1 container
				GinkgoWriter.Printf("Creating L1 container: %s (dynamic IP from host Garden pool)\n", l1Handle)
				err = l1Driver.Bootstrap()
				Expect(err).NotTo(HaveOccurred(), "Failed to bootstrap L1 container")

				// Configure and install Garden in L1
				// Use a different network pool than host Garden to avoid IP conflicts
				cfg := gardeninstaller.DefaultConfig()
				cfg.ReleaseTarballPath = releaseTarball
				cfg.Debug = true
				cfg.ListenAddress = fmt.Sprintf("0.0.0.0:%d", L1ContainerPort)
				cfg.NetworkPool = "10.253.0.0/22"            // L1 uses different pool than L0 (10.254.0.0/22)
				cfg.StoreSizeBytes = 35 * 1024 * 1024 * 1024 // 35GB for L1

				// CRITICAL: Disable containerd mode for nested Garden installations.
				// Containerd cannot run inside containers because it requires cgroups and
				// capabilities that are not available in nested environments.
				containerdMode := false
				cfg.ContainerdMode = &containerdMode

				l1Installer = gardeninstaller.New(cfg, l1Driver)

				GinkgoWriter.Printf("Installing Garden in L1 container...\n")
				err = l1Installer.Install()
				Expect(err).NotTo(HaveOccurred(), "Failed to install Garden in L1")

				GinkgoWriter.Printf("Starting Garden in L1 container...\n")
				err = l1Installer.Start()
				Expect(err).NotTo(HaveOccurred(), "Failed to start Garden in L1")

				// Wait for Garden to be ready
				time.Sleep(3 * time.Second)

				// Get L1 container IP for direct connection (bypasses NetIn port forwarding
				// which may not work on systems using nftables instead of iptables)
				l1ContainerIP, err := l1Driver.ContainerIP()
				Expect(err).NotTo(HaveOccurred(), "Failed to get L1 container IP")
				GinkgoWriter.Printf("L1 container IP: %s\n", l1ContainerIP)

				// Connect to L1 Garden using container IP directly
				l1GardenAddress = fmt.Sprintf("%s:%d", l1ContainerIP, L1ContainerPort)
			})

			It("can ping L1 Garden from host", func() {
				GinkgoWriter.Printf("Connecting to L1 Garden at %s\n", l1GardenAddress)

				// L1 Garden runs at a container IP (e.g., 10.254.0.2) which is only reachable
				// from within the agent VM's network namespace. We need to use an SSH client
				// connected to the agent VM (not just the jumpbox) to reach it.
				agentSSHClient, err := utils.DialAgentThroughJumpbox(agentIP)
				Expect(err).NotTo(HaveOccurred(), "Failed to connect to agent VM")

				l1GardenClient, err = installerdriver.NewGardenAPIClient(agentSSHClient, l1GardenAddress, nil)
				Expect(err).NotTo(HaveOccurred(), "Failed to connect to L1 Garden at %s", l1GardenAddress)

				err = l1GardenClient.Ping()
				Expect(err).NotTo(HaveOccurred(), "Failed to ping L1 Garden")

				GinkgoWriter.Printf("Successfully connected to L1 Garden\n")
			})

			It("can create container in L1 Garden", func() {
				Expect(l1GardenClient).NotTo(BeNil(), "L1 Garden client not initialized")

				// Create a simple test container in L1
				testHandle := fmt.Sprintf("l1-test-%d", time.Now().UnixNano())
				container, err := l1GardenClient.Create(garden.ContainerSpec{
					Handle: testHandle,
					Image:  garden.ImageRef{URI: utils.NobleStemcellImage},
				})
				Expect(err).NotTo(HaveOccurred(), "Failed to create container in L1 Garden")

				// Run a simple command
				process, err := container.Run(garden.ProcessSpec{
					Path: "echo",
					Args: []string{"Hello from L1 container"},
					User: "root",
				}, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
				exitCode, err := process.Wait()
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCode).To(Equal(0))

				// Clean up test container
				err = l1GardenClient.Destroy(testHandle)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("bosh-agent firewall in L1 container", Ordered, func() {
				var l1AgentDriver *installerdriver.GardenDriver
				var l1AgentInstaller *agentinstaller.Installer

				BeforeAll(func() {
					Expect(l1GardenClient).NotTo(BeNil(), "L1 Garden client not initialized")

					// Create a container in L1 Garden for running the agent
					agentHandle := fmt.Sprintf("l1-agent-%d", time.Now().UnixNano())

					l1AgentDriver = installerdriver.NewGardenDriver(installerdriver.GardenDriverConfig{
						GardenClient: l1GardenClient,
						ParentDriver: l1Driver,
						Handle:       agentHandle,
						Image:        utils.NobleStemcellImage,
					})

					err := l1AgentDriver.Bootstrap()
					Expect(err).NotTo(HaveOccurred(), "Failed to bootstrap agent container in L1")

					// Install agent using agentinstaller
					agentCfg := agentinstaller.DefaultConfig()
					agentCfg.AgentID = "test-agent-l1"
					agentCfg.Debug = true
					agentCfg.EnableNATSFirewall = true

					l1AgentInstaller = agentinstaller.New(agentCfg, l1AgentDriver)
					err = l1AgentInstaller.Install()
					Expect(err).NotTo(HaveOccurred(), "Failed to install agent in L1")

					// Verify nftables kernel support
					available, err := l1AgentInstaller.CheckNftablesKernelSupport()
					Expect(err).NotTo(HaveOccurred())
					if !available {
						Skip("nftables kernel support not available in L1 container")
					}

					// Start agent and wait for firewall rules
					GinkgoWriter.Printf("Starting bosh-agent in L1 container...\n")
					stdout, stderr, exitCode, err := l1AgentDriver.RunScript(fmt.Sprintf(`
%s -P ubuntu -C %s &
AGENT_PID=$!

for i in $(seq 1 20); do
	sleep 1
	if %s table inet bosh_agent 2>/dev/null | grep -q "monit_access"; then
		echo "Firewall rules created after ${i}s"
		break
	fi
	if [ $i -ge 15 ]; then
		echo "Timeout waiting for firewall rules"
		break
	fi
done

kill $AGENT_PID 2>/dev/null || true
sleep 1
echo "Agent completed"
`, l1AgentInstaller.AgentBinaryPath(), l1AgentInstaller.AgentConfigPath(), l1AgentInstaller.NftDumpBinaryPath()))
					if err != nil && !strings.Contains(err.Error(), "timed out") {
						Fail(fmt.Sprintf("Agent startup failed: %v, stdout: %s, stderr: %s", err, stdout, stderr))
					}
					_ = exitCode
					GinkgoWriter.Printf("Agent output: stdout=%s, stderr=%s\n", stdout, stderr)
				})

				AfterAll(func() {
					if l1AgentDriver != nil {
						_ = l1AgentDriver.Cleanup()
					}
				})

				It("creates bosh_agent firewall table in L1 container", func() {
					ruleOutput, err := l1AgentInstaller.NftDumpTable("inet", "bosh_agent")
					Expect(err).NotTo(HaveOccurred(), "Agent failed to create firewall table in L1")

					GinkgoWriter.Printf("L1 nftables rules:\n%s\n", ruleOutput)

					Expect(ruleOutput).To(ContainSubstring("family: inet"))
					Expect(ruleOutput).To(ContainSubstring("name: bosh_agent"))
					Expect(ruleOutput).To(ContainSubstring("name: monit_access"))
				})

				It("creates firewall rules with correct structure in L1", func() {
					ruleOutput, err := l1AgentInstaller.NftDumpTable("inet", "bosh_agent")
					Expect(err).NotTo(HaveOccurred())

					// Verify key rule components
					Expect(ruleOutput).To(ContainSubstring("dport 2822"))
					Expect(ruleOutput).To(ContainSubstring("mark set 0xb054"))
					Expect(ruleOutput).To(ContainSubstring("accept"))
				})
			})
		})

		// Level 2: Garden inside L1 Garden container (3 levels of nesting)
		Context("Level 2: Garden inside L1 Garden container", Ordered, func() {
			BeforeAll(func() {
				// Skip if L1 Garden is not available
				if l1GardenClient == nil {
					Skip("L1 Garden not initialized - run L1 tests first")
				}

				// Create L2 container handle
				l2Handle := fmt.Sprintf("l2-garden-%d", time.Now().UnixNano())

				// Create L2 GardenDriver with L1Driver as parent
				// Let Garden dynamically allocate an IP from L1's pool (10.253.0.0/22)
				l2Driver = installerdriver.NewGardenDriver(installerdriver.GardenDriverConfig{
					GardenClient: l1GardenClient,
					ParentDriver: l1Driver,
					Handle:       l2Handle,
					Image:        utils.NobleStemcellImage,
					// Network is empty - let Garden allocate from its pool
					DiskLimit: defaultDiskLimit,
				})

				// Bootstrap L2 container
				GinkgoWriter.Printf("Creating L2 container: %s (dynamic IP from L1 Garden pool)\n", l2Handle)

				// Collect diagnostics before L2 creation
				collectL1Diagnostics(l1Driver, "PRE-L2-BOOTSTRAP")

				err := l2Driver.Bootstrap()
				if err != nil {
					collectL1Diagnostics(l1Driver, "POST-L2-BOOTSTRAP-FAILURE")
				}
				Expect(err).NotTo(HaveOccurred(), "Failed to bootstrap L2 container")

				// Configure and install Garden in L2
				// Use a different network pool than L1 Garden to avoid IP conflicts
				cfg := gardeninstaller.DefaultConfig()
				cfg.ReleaseTarballPath = releaseTarball
				cfg.Debug = true
				cfg.ListenAddress = fmt.Sprintf("0.0.0.0:%d", L2ContainerPort)
				cfg.NetworkPool = "10.252.0.0/22"            // L2 uses different pool than L1 (10.253.0.0/22)
				cfg.StoreSizeBytes = 15 * 1024 * 1024 * 1024 // 15GB for L2

				// CRITICAL: Disable containerd mode for nested Garden installations.
				// Containerd cannot run inside containers because it requires cgroups and
				// capabilities that are not available in nested environments.
				containerdMode := false
				cfg.ContainerdMode = &containerdMode

				l2Installer = gardeninstaller.New(cfg, l2Driver)

				GinkgoWriter.Printf("Installing Garden in L2 container...\n")
				err = l2Installer.Install()
				if err != nil {
					collectL1Diagnostics(l1Driver, "POST-L2-INSTALL-FAILURE")
				}
				Expect(err).NotTo(HaveOccurred(), "Failed to install Garden in L2")

				GinkgoWriter.Printf("Starting Garden in L2 container...\n")
				err = l2Installer.Start()
				Expect(err).NotTo(HaveOccurred(), "Failed to start Garden in L2")

				// Wait for Garden to be ready
				time.Sleep(3 * time.Second)

				// Get L2 container IP
				l2ContainerIP, err := l2Driver.ContainerIP()
				Expect(err).NotTo(HaveOccurred(), "Failed to get L2 container IP")
				GinkgoWriter.Printf("L2 container IP: %s\n", l2ContainerIP)

				// L2 container is only reachable from within L1, not from the agent VM.
				// We need to start a TCP forwarder in L1 to reach L2 Garden.
				// The forwarder listens on L1's IP:7778 and forwards to L2's IP:7777.
				l2TargetAddr := fmt.Sprintf("%s:%d", l2ContainerIP, L2ContainerPort)
				forwarderAddr, err := installerdriver.StartTCPForwarder(l1Driver, 7778, l2TargetAddr)
				Expect(err).NotTo(HaveOccurred(), "Failed to start TCP forwarder in L1")
				GinkgoWriter.Printf("L2 forwarder started: %s -> %s\n", forwarderAddr, l2TargetAddr)

				// Connect to L2 Garden through the forwarder in L1
				l2GardenAddress = forwarderAddr
			})

			AfterAll(func() {
				if l2Installer != nil {
					GinkgoWriter.Printf("Stopping L2 Garden...\n")
					if err := l2Installer.Stop(); err != nil {
						GinkgoWriter.Printf("Warning: failed to stop L2 Garden: %v\n", err)
					}
				}

				if l2Driver != nil {
					GinkgoWriter.Printf("Cleaning up L2 container...\n")
					if err := l2Driver.Cleanup(); err != nil {
						GinkgoWriter.Printf("Warning: failed to cleanup L2 container: %v\n", err)
					}
				}
			})

			It("can ping L2 Garden from host", func() {
				GinkgoWriter.Printf("Connecting to L2 Garden at %s\n", l2GardenAddress)

				// L2 Garden is accessed through a TCP forwarder running in L1 container.
				// The forwarder address is at L1's container IP, which is only reachable
				// from within the agent VM's network namespace.
				agentSSHClient, err := utils.DialAgentThroughJumpbox(agentIP)
				Expect(err).NotTo(HaveOccurred(), "Failed to connect to agent VM")

				l2GardenClient, err = installerdriver.NewGardenAPIClient(agentSSHClient, l2GardenAddress, nil)
				Expect(err).NotTo(HaveOccurred(), "Failed to connect to L2 Garden at %s", l2GardenAddress)

				err = l2GardenClient.Ping()
				Expect(err).NotTo(HaveOccurred(), "Failed to ping L2 Garden")

				GinkgoWriter.Printf("Successfully connected to L2 Garden (3 levels deep!)\n")
			})

			It("can create container in L2 Garden", func() {
				Expect(l2GardenClient).NotTo(BeNil(), "L2 Garden client not initialized")

				// Create a simple test container in L2 using local busybox rootfs
				testHandle := fmt.Sprintf("l2-test-%d", time.Now().UnixNano())
				container, err := l2GardenClient.Create(garden.ContainerSpec{
					Handle: testHandle,
					// Empty Image means use Garden's default rootfs (busybox)
				})
				Expect(err).NotTo(HaveOccurred(), "Failed to create container in L2 Garden")

				// Run a simple command (busybox uses /bin/sh -c for echo)
				process, err := container.Run(garden.ProcessSpec{
					Path: "/bin/sh",
					Args: []string{"-c", "echo 'Hello from L2 container (3 levels deep!)'"},
					User: "root",
				}, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
				exitCode, err := process.Wait()
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCode).To(Equal(0))

				// Clean up test container
				err = l2GardenClient.Destroy(testHandle)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("bosh-agent firewall in L2 container (deepest nesting)", Ordered, func() {
				var l2AgentDriver *installerdriver.GardenDriver
				var l2AgentInstaller *agentinstaller.Installer

				BeforeAll(func() {
					Expect(l2GardenClient).NotTo(BeNil(), "L2 Garden client not initialized")

					// Create a container in L2 Garden for running the agent
					agentHandle := fmt.Sprintf("l2-agent-%d", time.Now().UnixNano())

					l2AgentDriver = installerdriver.NewGardenDriver(installerdriver.GardenDriverConfig{
						GardenClient: l2GardenClient,
						ParentDriver: l2Driver,
						Handle:       agentHandle,
						Image:        utils.NobleStemcellImage,
					})

					err := l2AgentDriver.Bootstrap()
					Expect(err).NotTo(HaveOccurred(), "Failed to bootstrap agent container in L2")

					// Install agent using agentinstaller
					agentCfg := agentinstaller.DefaultConfig()
					agentCfg.AgentID = "test-agent-l2"
					agentCfg.Debug = true
					agentCfg.EnableNATSFirewall = true

					l2AgentInstaller = agentinstaller.New(agentCfg, l2AgentDriver)
					err = l2AgentInstaller.Install()
					Expect(err).NotTo(HaveOccurred(), "Failed to install agent in L2")

					// Verify nftables kernel support at deepest nesting level
					available, err := l2AgentInstaller.CheckNftablesKernelSupport()
					Expect(err).NotTo(HaveOccurred())
					if !available {
						Skip("nftables kernel support not available in L2 container")
					}
					GinkgoWriter.Printf("nftables kernel support confirmed at L2 (3 levels deep)\n")

					// Start agent and wait for firewall rules
					GinkgoWriter.Printf("Starting bosh-agent in L2 container (3 levels deep)...\n")
					stdout, stderr, exitCode, err := l2AgentDriver.RunScript(fmt.Sprintf(`
%s -P ubuntu -C %s &
AGENT_PID=$!

for i in $(seq 1 20); do
	sleep 1
	if %s table inet bosh_agent 2>/dev/null | grep -q "monit_access"; then
		echo "Firewall rules created after ${i}s"
		break
	fi
	if [ $i -ge 15 ]; then
		echo "Timeout waiting for firewall rules"
		break
	fi
done

kill $AGENT_PID 2>/dev/null || true
sleep 1
echo "Agent completed"
`, l2AgentInstaller.AgentBinaryPath(), l2AgentInstaller.AgentConfigPath(), l2AgentInstaller.NftDumpBinaryPath()))
					if err != nil && !strings.Contains(err.Error(), "timed out") {
						Fail(fmt.Sprintf("Agent startup failed in L2: %v, stdout: %s, stderr: %s", err, stdout, stderr))
					}
					_ = exitCode
					GinkgoWriter.Printf("L2 Agent output: stdout=%s, stderr=%s\n", stdout, stderr)
				})

				AfterAll(func() {
					if l2AgentDriver != nil {
						_ = l2AgentDriver.Cleanup()
					}
				})

				It("creates bosh_agent firewall table in L2 container (3 levels deep)", func() {
					ruleOutput, err := l2AgentInstaller.NftDumpTable("inet", "bosh_agent")
					Expect(err).NotTo(HaveOccurred(), "Agent failed to create firewall table in L2 (3 levels deep)")

					GinkgoWriter.Printf("L2 nftables rules (3 levels deep):\n%s\n", ruleOutput)

					Expect(ruleOutput).To(ContainSubstring("family: inet"))
					Expect(ruleOutput).To(ContainSubstring("name: bosh_agent"))
					Expect(ruleOutput).To(ContainSubstring("name: monit_access"))
				})

				It("creates firewall rules with correct structure in L2", func() {
					ruleOutput, err := l2AgentInstaller.NftDumpTable("inet", "bosh_agent")
					Expect(err).NotTo(HaveOccurred())

					// Verify key rule components - same validation as L1
					Expect(ruleOutput).To(ContainSubstring("dport 2822"))
					Expect(ruleOutput).To(ContainSubstring("mark set 0xb054"))
					Expect(ruleOutput).To(ContainSubstring("accept"))

					GinkgoWriter.Printf("SUCCESS: Firewall rules verified at 3 levels of nesting!\n")
					GinkgoWriter.Printf("This validates the Concourse -> start-bosh.sh -> bosh-lite scenario.\n")
				})
			})
		})
	})
})
