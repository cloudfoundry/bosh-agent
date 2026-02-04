// Package utils provides test utilities for Garden integration tests.
package utils

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	windowsutils "github.com/cloudfoundry/bosh-agent/v2/integration/windows/utils"
)

const (
	// NobleStemcellImage is the OCI image for Ubuntu Noble stemcell
	NobleStemcellImage = "docker://ghcr.io/cloudfoundry/ubuntu-noble-stemcell:latest"
	// JammyStemcellImage is the OCI image for Ubuntu Jammy stemcell
	JammyStemcellImage = "docker://ghcr.io/cloudfoundry/ubuntu-jammy-stemcell:latest"
	// DefaultStemcellImage is the default OCI image to use for creating containers
	DefaultStemcellImage = NobleStemcellImage
)

// NestedGardenDataDir is the base directory on the host for nested container data.
// Each nested container gets a subdirectory here that is bind-mounted to /var/vcap/data.
// This provides access to the host's data disk (100+ GB) instead of the container's
// constrained overlay filesystem (~10GB).
const NestedGardenDataDir = "/var/vcap/data/nested-garden-test"

// NftDumpBinaryPath is the path where the nft-dump binary is installed in containers.
const NftDumpBinaryPath = "/var/vcap/bosh/bin/nft-dump"

// GardenAddress returns the Garden server address from environment.
// Returns empty string if not set.
func GardenAddress() string {
	return os.Getenv("GARDEN_ADDRESS")
}

// StemcellImage returns the OCI stemcell image to use.
// Uses STEMCELL_IMAGE env var if set, otherwise returns DefaultStemcellImage.
func StemcellImage() string {
	if img := os.Getenv("STEMCELL_IMAGE"); img != "" {
		return img
	}
	return DefaultStemcellImage
}

// AllStemcellImages returns the list of stemcell images to test.
// If STEMCELL_IMAGE env var is set, returns only that image.
// Otherwise returns both Noble and Jammy images.
func AllStemcellImages() []string {
	if img := os.Getenv("STEMCELL_IMAGE"); img != "" {
		return []string{img}
	}
	return []string{NobleStemcellImage, JammyStemcellImage}
}

// StemcellImageName extracts a short name from the full image URI for logging.
// e.g., "docker://ghcr.io/cloudfoundry/ubuntu-noble-stemcell:all" -> "ubuntu-noble-stemcell"
func StemcellImageName(image string) string {
	// Remove docker:// prefix
	name := strings.TrimPrefix(image, "docker://")
	// Remove registry prefix (everything before last /)
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}
	// Remove tag suffix
	if idx := strings.Index(name, ":"); idx != -1 {
		name = name[:idx]
	}
	return name
}

// GetAgentIP returns the agent IP from environment or extracts it from GARDEN_ADDRESS.
func GetAgentIP() string {
	agentIP := os.Getenv("AGENT_IP")
	if agentIP != "" {
		return agentIP
	}

	// Try to extract from GARDEN_ADDRESS
	gardenAddr := GardenAddress()
	if gardenAddr != "" {
		if idx := strings.LastIndex(gardenAddr, ":"); idx != -1 {
			return gardenAddr[:idx]
		}
	}
	return ""
}

// GetReleaseTarball returns the path to the Garden release tarball from environment.
func GetReleaseTarball() string {
	return os.Getenv("GARDEN_RELEASE_TARBALL")
}

// AgentKeyPaths returns common paths to check for SSH keys when connecting to agents.
func AgentKeyPaths() []string {
	home := os.Getenv("HOME")
	return []string{
		os.Getenv("AGENT_KEY_PATH"),
		"debug-ssh-key",
		"../../debug-ssh-key",
		home + "/.ssh/id_rsa",
		home + "/.ssh/id_ed25519",
	}
}

// FindAgentKey searches for an SSH key to use when connecting to agents.
func FindAgentKey() string {
	for _, path := range AgentKeyPaths() {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// FindFile returns the first path that exists from the given list.
func FindFile(paths []string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// FindAgentBinary searches for the bosh-agent binary in common locations.
func FindAgentBinary() string {
	return FindFile([]string{
		"bosh-agent-linux-amd64",
		"../../bosh-agent-linux-amd64",
	})
}

// FindNftDumpBinary searches for the nft-dump binary in common locations.
func FindNftDumpBinary() string {
	return FindFile([]string{
		"nft-dump-linux-amd64",
		"../../nft-dump-linux-amd64",
	})
}

// GetAgentUser returns the SSH user for the agent VM from environment.
// Defaults to "root" if not set.
func GetAgentUser() string {
	user := os.Getenv("AGENT_USER")
	if user == "" {
		return "root"
	}
	return user
}

// DialAgentThroughJumpbox connects to the agent VM through the jumpbox SSH tunnel.
// This is the proper way to establish an SSH connection to the agent VM when
// running tests from outside the deployment network.
func DialAgentThroughJumpbox(agentIP string) (*ssh.Client, error) {
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
	agentKeyPath := FindAgentKey()
	if agentKeyPath == "" {
		conn.Close()
		return nil, fmt.Errorf("no agent SSH key found - set AGENT_KEY_PATH")
	}

	keyData, err := os.ReadFile(agentKeyPath)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read agent SSH key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to parse agent SSH key: %w", err)
	}

	// Get agent username
	agentUser := GetAgentUser()

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
