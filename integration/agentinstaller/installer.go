// Package agentinstaller provides utilities for installing and configuring
// the bosh-agent on any target environment using the installerdriver.Driver interface.
//
// This package is used in integration tests to set up bosh-agent in containers
// (both at the VM level and inside nested Garden containers).
package agentinstaller

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cloudfoundry/bosh-agent/v2/integration/installerdriver"
)

// Config holds the configuration for installing the bosh-agent.
type Config struct {
	// AgentBinaryPath is the path to the bosh-agent binary (local path).
	// If empty, common locations will be searched.
	AgentBinaryPath string

	// NftDumpBinaryPath is the path to the nft-dump binary (local path).
	// If empty, common locations will be searched and it will be built if not found.
	NftDumpBinaryPath string

	// AgentID is the agent ID to use in settings.
	AgentID string

	// MbusURL is the message bus URL for the agent.
	MbusURL string

	// EnableNATSFirewall enables the nftables firewall feature.
	EnableNATSFirewall bool

	// BaseDir is the BOSH installation directory on the target (default: /var/vcap).
	BaseDir string

	// Debug enables debug logging during installation.
	Debug bool
}

// DefaultConfig returns a Config with sensible defaults for testing.
func DefaultConfig() Config {
	return Config{
		AgentID:            "test-agent",
		MbusURL:            "https://mbus:mbus@127.0.0.1:6868",
		EnableNATSFirewall: true,
		BaseDir:            "/var/vcap",
		Debug:              false,
	}
}

// Installer installs and configures the bosh-agent on a target environment.
type Installer struct {
	cfg    Config
	driver installerdriver.Driver
}

// New creates a new Installer with the given configuration and driver.
func New(cfg Config, driver installerdriver.Driver) *Installer {
	return &Installer{cfg: cfg, driver: driver}
}

// Install prepares the environment and installs the bosh-agent.
// It performs the following steps:
// 1. Create required directories
// 2. Copy and configure bosh-agent binary
// 3. Copy nft-dump binary (for nftables testing)
// 4. Generate agent configuration files
// 5. Create dummy bosh-agent-rc script
func (i *Installer) Install() error {
	if !i.driver.IsBootstrapped() {
		return fmt.Errorf("driver not bootstrapped: call driver.Bootstrap() before installer.Install()")
	}

	i.log("Installing bosh-agent to %s", i.driver.Description())

	// Step 1: Create directories
	if err := i.createDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Step 2: Copy bosh-agent binary
	if err := i.installAgentBinary(); err != nil {
		return fmt.Errorf("failed to install agent binary: %w", err)
	}

	// Step 3: Copy nft-dump binary
	if err := i.installNftDump(); err != nil {
		return fmt.Errorf("failed to install nft-dump: %w", err)
	}

	// Step 4: Generate configuration files
	if err := i.generateConfigs(); err != nil {
		return fmt.Errorf("failed to generate configs: %w", err)
	}

	// Step 5: Create dummy bosh-agent-rc
	if err := i.createAgentRC(); err != nil {
		return fmt.Errorf("failed to create bosh-agent-rc: %w", err)
	}

	i.log("bosh-agent installation complete on %s", i.driver.Description())
	return nil
}

// createDirectories creates the required directory structure on the target.
func (i *Installer) createDirectories() error {
	dirs := []string{
		filepath.Join(i.cfg.BaseDir, "bosh", "bin"),
		filepath.Join(i.cfg.BaseDir, "bosh", "log"),
		filepath.Join(i.cfg.BaseDir, "data"),
		filepath.Join(i.cfg.BaseDir, "data", "sys"),
		filepath.Join(i.cfg.BaseDir, "data", "blobs"),
		filepath.Join(i.cfg.BaseDir, "monit", "job"),
	}

	for _, dir := range dirs {
		i.log("Creating directory: %s", dir)
		if err := i.driver.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	return nil
}

// installAgentBinary finds and copies the bosh-agent binary to the target.
func (i *Installer) installAgentBinary() error {
	binaryPath := i.cfg.AgentBinaryPath
	if binaryPath == "" {
		binaryPath = findFile([]string{
			"bosh-agent-linux-amd64",
			"../../bosh-agent-linux-amd64",
		})
	}

	if binaryPath == "" {
		return fmt.Errorf("bosh-agent binary not found - build it with 'bin/build-linux-amd64' or set AgentBinaryPath")
	}

	i.log("Installing agent binary from %s", binaryPath)

	// Read the binary
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to read agent binary: %w", err)
	}

	// Write to target
	targetPath := filepath.Join(i.cfg.BaseDir, "bosh", "bin", "bosh-agent")
	if err := i.driver.WriteFile(targetPath, data, 0755); err != nil {
		return fmt.Errorf("failed to write agent binary: %w", err)
	}

	return nil
}

// installNftDump finds, builds if necessary, and copies the nft-dump binary.
func (i *Installer) installNftDump() error {
	binaryPath := i.cfg.NftDumpBinaryPath
	if binaryPath == "" {
		binaryPath = findFile([]string{
			"nft-dump-linux-amd64",
			"../../nft-dump-linux-amd64",
		})
	}

	// Build if not found
	if binaryPath == "" {
		i.log("nft-dump binary not found, building it...")

		sourcePaths := []string{
			"./integration/nftdump",
			"../../integration/nftdump",
			"../nftdump",
			"./nftdump",
		}

		var sourceDir string
		for _, sp := range sourcePaths {
			if _, err := os.Stat(filepath.Join(sp, "main.go")); err == nil {
				sourceDir = sp
				break
			}
		}

		if sourceDir == "" {
			return fmt.Errorf("nft-dump source not found - cannot build")
		}

		outputPath := "nft-dump-linux-amd64"
		cmd := exec.Command("go", "build", "-o", outputPath, sourceDir)
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to build nft-dump: %w, output: %s", err, string(output))
		}
		i.log("Built nft-dump binary: %s", outputPath)
		binaryPath = outputPath
	}

	i.log("Installing nft-dump from %s", binaryPath)

	// Read the binary
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to read nft-dump binary: %w", err)
	}

	// Write to target
	targetPath := filepath.Join(i.cfg.BaseDir, "bosh", "bin", "nft-dump")
	if err := i.driver.WriteFile(targetPath, data, 0755); err != nil {
		return fmt.Errorf("failed to write nft-dump binary: %w", err)
	}

	return nil
}

// generateConfigs creates the agent configuration files.
func (i *Installer) generateConfigs() error {
	// Create agent.json
	agentConfig := map[string]interface{}{
		"Infrastructure": map[string]interface{}{
			"Settings": map[string]interface{}{
				"Sources": []map[string]interface{}{
					{
						"Type":         "File",
						"SettingsPath": filepath.Join(i.cfg.BaseDir, "bosh", "settings.json"),
					},
				},
			},
		},
		"Platform": map[string]interface{}{
			"Linux": map[string]interface{}{
				"EnableNATSFirewall": i.cfg.EnableNATSFirewall,
			},
		},
	}

	agentJSON, err := json.MarshalIndent(agentConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal agent config: %w", err)
	}

	agentConfigPath := filepath.Join(i.cfg.BaseDir, "bosh", "agent.json")
	if err := i.driver.WriteFile(agentConfigPath, agentJSON, 0644); err != nil {
		return fmt.Errorf("failed to write agent.json: %w", err)
	}
	i.log("Created agent.json")

	// Create settings.json
	settings := map[string]interface{}{
		"agent_id": i.cfg.AgentID,
		"mbus":     i.cfg.MbusURL,
		"ntp":      []string{},
		"blobstore": map[string]interface{}{
			"provider": "local",
			"options": map[string]interface{}{
				"blobstore_path": filepath.Join(i.cfg.BaseDir, "data", "blobs"),
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
			"name": "test-vm",
		},
		"env": map[string]interface{}{
			"bosh": map[string]interface{}{
				"mbus": map[string]interface{}{
					"urls": []string{i.cfg.MbusURL},
				},
			},
		},
	}

	settingsJSON, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	settingsPath := filepath.Join(i.cfg.BaseDir, "bosh", "settings.json")
	if err := i.driver.WriteFile(settingsPath, settingsJSON, 0644); err != nil {
		return fmt.Errorf("failed to write settings.json: %w", err)
	}
	i.log("Created settings.json")

	return nil
}

// createAgentRC creates a dummy bosh-agent-rc script.
func (i *Installer) createAgentRC() error {
	script := []byte("#!/bin/bash\nexit 0\n")
	scriptPath := "/usr/local/bin/bosh-agent-rc"

	// Ensure parent directory exists
	if err := i.driver.MkdirAll("/usr/local/bin", 0755); err != nil {
		return fmt.Errorf("failed to create /usr/local/bin: %w", err)
	}

	if err := i.driver.WriteFile(scriptPath, script, 0755); err != nil {
		return fmt.Errorf("failed to write bosh-agent-rc: %w", err)
	}
	i.log("Created bosh-agent-rc")

	return nil
}

// NftDumpBinaryPath returns the path to the nft-dump binary on the target.
func (i *Installer) NftDumpBinaryPath() string {
	return filepath.Join(i.cfg.BaseDir, "bosh", "bin", "nft-dump")
}

// AgentBinaryPath returns the path to the bosh-agent binary on the target.
func (i *Installer) AgentBinaryPath() string {
	return filepath.Join(i.cfg.BaseDir, "bosh", "bin", "bosh-agent")
}

// AgentConfigPath returns the path to the agent.json config file on the target.
func (i *Installer) AgentConfigPath() string {
	return filepath.Join(i.cfg.BaseDir, "bosh", "agent.json")
}

func (i *Installer) log(format string, args ...interface{}) {
	if i.cfg.Debug {
		fmt.Printf("[agentinstaller] "+format+"\n", args...)
	}
}

// findFile returns the first path that exists from the given list.
func findFile(paths []string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// CheckNftablesKernelSupport checks if the kernel supports nftables.
// Uses the nft-dump utility installed by Install().
func (i *Installer) CheckNftablesKernelSupport() (bool, error) {
	nftDumpPath := i.NftDumpBinaryPath()
	_, _, exitCode, err := i.driver.RunCommand(nftDumpPath, "check")
	if err != nil {
		return false, err
	}
	return exitCode == 0, nil
}

// NftDumpTable returns YAML output for a specific nftables table.
func (i *Installer) NftDumpTable(family, name string) (string, error) {
	nftDumpPath := i.NftDumpBinaryPath()
	stdout, stderr, exitCode, err := i.driver.RunCommand(nftDumpPath, "table", family, name)
	if err != nil {
		return "", fmt.Errorf("failed to run nft-dump table: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("nft-dump table failed: exit %d, stderr: %s", exitCode, stderr)
	}
	return stdout, nil
}

// NftDumpTables returns YAML output listing all nftables tables.
func (i *Installer) NftDumpTables() (string, error) {
	nftDumpPath := i.NftDumpBinaryPath()
	stdout, stderr, exitCode, err := i.driver.RunCommand(nftDumpPath, "tables")
	if err != nil {
		return "", fmt.Errorf("failed to run nft-dump tables: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("nft-dump tables failed: exit %d, stderr: %s", exitCode, stderr)
	}
	return stdout, nil
}
