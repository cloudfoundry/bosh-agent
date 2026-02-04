package garden_test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/cloudfoundry/bosh-agent/v2/integration/gardeninstaller"
	"github.com/cloudfoundry/bosh-agent/v2/integration/installerdriver"
	"github.com/cloudfoundry/bosh-agent/v2/integration/utils"
	windowsutils "github.com/cloudfoundry/bosh-agent/v2/integration/windows/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// gardenInstallerInstance holds the installer for cleanup
var gardenInstallerInstance *gardeninstaller.Installer

// vmDriver holds the SSH driver for the agent VM
var vmDriver *installerdriver.SSHDriver

// agentSSHClient holds the SSH connection to the agent for cleanup
var agentSSHClient *ssh.Client

func TestGarden(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Garden Integration Suite")
}

var _ = BeforeSuite(func() {
	// Check if Garden is already available
	gardenAddr := utils.GardenAddress()
	if gardenAddr != "" {
		GinkgoWriter.Printf("Using existing Garden at %s\n", gardenAddr)

		// Verify connectivity by pinging Garden through SSH tunnel
		sshClient, err := windowsutils.GetSSHTunnelClient()
		if err != nil {
			GinkgoWriter.Printf("Warning: Could not get SSH tunnel client: %v\n", err)
			GinkgoWriter.Printf("Will attempt to install Garden if GARDEN_RELEASE_TARBALL is set\n")
		} else {
			gardenClient, err := installerdriver.NewGardenAPIClient(sshClient, gardenAddr, nil)
			if err != nil {
				GinkgoWriter.Printf("Warning: Could not connect to Garden at %s: %v\n", gardenAddr, err)
				GinkgoWriter.Printf("Will attempt to install Garden if GARDEN_RELEASE_TARBALL is set\n")
			} else if err := gardenClient.Ping(); err != nil {
				GinkgoWriter.Printf("Warning: Garden ping failed at %s: %v\n", gardenAddr, err)
				GinkgoWriter.Printf("Will attempt to install Garden if GARDEN_RELEASE_TARBALL is set\n")
			} else {
				GinkgoWriter.Printf("Garden connectivity verified\n")
				return
			}
		}
	}

	// Check if we should install Garden from a compiled release
	releaseTarball := utils.GetReleaseTarball()
	if releaseTarball == "" {
		if gardenAddr == "" {
			Skip("GARDEN_ADDRESS not set and GARDEN_RELEASE_TARBALL not provided - skipping Garden tests")
		}
		return
	}

	// Verify the tarball exists
	if _, err := os.Stat(releaseTarball); err != nil {
		Fail("GARDEN_RELEASE_TARBALL does not exist: " + releaseTarball)
	}

	GinkgoWriter.Printf("Installing Garden from tarball: %s\n", releaseTarball)

	// Get agent IP
	agentIP := utils.GetAgentIP()
	if agentIP == "" {
		Fail("AGENT_IP must be set when using GARDEN_RELEASE_TARBALL")
	}

	// Connect to agent VM through jumpbox
	var err error
	agentSSHClient, err = dialAgentThroughJumpbox(agentIP)
	if err != nil {
		Fail("Failed to connect to agent VM: " + err.Error())
	}

	// Create SSH driver for the VM
	vmDriver = installerdriver.NewSSHDriver(installerdriver.SSHDriverConfig{
		Client:  agentSSHClient,
		Host:    agentIP,
		UseSudo: true,
	})

	// Bootstrap the driver (creates base directories)
	err = vmDriver.Bootstrap()
	Expect(err).NotTo(HaveOccurred(), "Failed to bootstrap VM driver")

	// Configure the installer
	cfg := gardeninstaller.DefaultConfig()
	cfg.ReleaseTarballPath = releaseTarball
	cfg.Debug = true

	// Determine listen address
	listenAddr := os.Getenv("GARDEN_LISTEN_ADDRESS")
	if listenAddr == "" {
		listenAddr = "0.0.0.0:7777"
	}
	cfg.ListenAddress = listenAddr

	// Create installer
	gardenInstallerInstance = gardeninstaller.New(cfg, vmDriver)

	// Install Garden
	GinkgoWriter.Printf("Installing Garden on %s...\n", agentIP)
	err = gardenInstallerInstance.Install()
	Expect(err).NotTo(HaveOccurred(), "Failed to install Garden")

	// Start Garden
	GinkgoWriter.Printf("Starting Garden...\n")
	err = gardenInstallerInstance.Start()
	Expect(err).NotTo(HaveOccurred(), "Failed to start Garden")

	// Set GARDEN_ADDRESS for the tests if not already set
	if os.Getenv("GARDEN_ADDRESS") == "" {
		// Extract port from listen address
		port := "7777"
		if len(listenAddr) > 2 {
			if idx := lastIndexByte(listenAddr, ':'); idx != -1 {
				port = listenAddr[idx+1:]
			}
		}
		os.Setenv("GARDEN_ADDRESS", agentIP+":"+port)
		GinkgoWriter.Printf("Set GARDEN_ADDRESS=%s\n", os.Getenv("GARDEN_ADDRESS"))
	}

	GinkgoWriter.Printf("Garden installed and started successfully\n")
})

var _ = AfterSuite(func() {
	if gardenInstallerInstance != nil {
		GinkgoWriter.Printf("Stopping Garden...\n")
		if err := gardenInstallerInstance.Stop(); err != nil {
			GinkgoWriter.Printf("Warning: failed to stop Garden: %v\n", err)
		}
	}

	if vmDriver != nil {
		vmDriver.Cleanup()
	}

	if agentSSHClient != nil {
		agentSSHClient.Close()
	}
})

// dialAgentThroughJumpbox connects to the agent VM through the jumpbox SSH tunnel
func dialAgentThroughJumpbox(agentIP string) (*ssh.Client, error) {
	// Get jumpbox connection
	jumpboxClient, err := windowsutils.GetSSHTunnelClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to jumpbox: %w", err)
	}

	// Dial the agent through the jumpbox
	conn, err := jumpboxClient.Dial("tcp", fmt.Sprintf("%s:22", agentIP))
	if err != nil {
		return nil, fmt.Errorf("failed to dial agent through jumpbox: %w", err)
	}

	// Get agent SSH credentials
	agentKeyPath := utils.FindAgentKey()
	if agentKeyPath == "" {
		return nil, fmt.Errorf("no agent SSH key found - set AGENT_KEY_PATH")
	}

	keyData, err := os.ReadFile(agentKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent SSH key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent SSH key: %w", err)
	}

	// Get agent username (default to root for BOSH VMs)
	agentUser := os.Getenv("AGENT_USER")
	if agentUser == "" {
		agentUser = "root"
	}

	// Create SSH client config
	sshConfig := &ssh.ClientConfig{
		User:            agentUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// Create SSH client connection over the tunneled connection
	nConn, chans, reqs, err := ssh.NewClientConn(conn, fmt.Sprintf("%s:22", agentIP), sshConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to establish SSH connection to agent: %w", err)
	}

	return ssh.NewClient(nConn, chans, reqs), nil
}

// dialAgentDirect connects directly to the agent VM (for local testing)
func dialAgentDirect(agentIP string, keyPath string) (*ssh.Client, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH key: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(agentIP, "22")
	return ssh.Dial("tcp", addr, sshConfig)
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
