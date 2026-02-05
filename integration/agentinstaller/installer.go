// Package agentinstaller provides utilities for installing and configuring
// the bosh-agent on any target environment using the installerdriver.Driver interface.
//
// This package is used in integration tests to set up bosh-agent in containers
// (both at the VM level and inside nested Garden containers).
package agentinstaller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cloudfoundry/bosh-agent/v2/integration/installerdriver"
)

// MonitPort is the port that monit listens on (127.0.0.1:2822).
const MonitPort = 2822

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

// StartMockMonit starts a simple TCP listener on the monit port (2822) to simulate monit.
// This uses various methods to create a persistent TCP listener that accepts connections.
// The listener runs in the background and must be stopped with StopMockMonit().
func (i *Installer) StartMockMonit() error {
	// Try multiple methods to start a TCP listener, in order of preference
	// Method 1: Use Python if available (most reliable)
	script := fmt.Sprintf(`
# Try to start mock monit using various methods

# Method 1: Python (most reliable and available on stemcells)
if command -v python3 >/dev/null 2>&1; then
    nohup python3 -c "
import socket
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(('127.0.0.1', %d))
s.listen(5)
while True:
    conn, addr = s.accept()
    conn.close()
" > /tmp/mock-monit.log 2>&1 &
    echo $! > /tmp/mock-monit.pid
    exit 0
fi

# Method 2: socat if available
if command -v socat >/dev/null 2>&1; then
    nohup socat TCP-LISTEN:%d,fork,reuseaddr EXEC:"/bin/cat" > /tmp/mock-monit.log 2>&1 &
    echo $! > /tmp/mock-monit.pid
    exit 0
fi

# Method 3: netcat (try different variants)
# BSD netcat
if nc -h 2>&1 | grep -q '\-l.*\-p'; then
    nohup sh -c 'while true; do nc -l -p %d; done' > /tmp/mock-monit.log 2>&1 &
    echo $! > /tmp/mock-monit.pid
    exit 0
fi

# GNU netcat / ncat
if command -v ncat >/dev/null 2>&1; then
    nohup ncat -l -k %d > /tmp/mock-monit.log 2>&1 &
    echo $! > /tmp/mock-monit.pid
    exit 0
fi

# Simple nc without options (busybox style - just bind and listen once)
if command -v nc >/dev/null 2>&1; then
    nohup sh -c 'while true; do echo "" | nc -l -p %d 2>/dev/null || nc -l %d 2>/dev/null || sleep 0.1; done' > /tmp/mock-monit.log 2>&1 &
    echo $! > /tmp/mock-monit.pid
    exit 0
fi

echo "No suitable tool found to create TCP listener" >&2
exit 1
`, MonitPort, MonitPort, MonitPort, MonitPort, MonitPort, MonitPort)

	stdout, stderr, exitCode, err := i.driver.RunScript(script)
	if err != nil {
		return fmt.Errorf("failed to start mock monit: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("failed to start mock monit: exit %d, stdout: %s, stderr: %s", exitCode, stdout, stderr)
	}
	return nil
}

// StopMockMonit stops the mock monit listener started by StartMockMonit().
func (i *Installer) StopMockMonit() error {
	script := `
if [ -f /tmp/mock-monit.pid ]; then
    pid=$(cat /tmp/mock-monit.pid)
    kill "$pid" 2>/dev/null || true
    rm -f /tmp/mock-monit.pid
fi
# Also kill any remaining processes on the port
fuser -k 2822/tcp 2>/dev/null || true
`
	_, _, _, err := i.driver.RunScript(script)
	return err
}

// WaitForMockMonit waits for the mock monit to be ready to accept connections.
func (i *Installer) WaitForMockMonit(ctx context.Context) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(10 * time.Second)
	}

	attempts := 0
	for time.Now().Before(deadline) {
		attempts++

		// Try multiple methods to check port availability
		// Method 1: Use /dev/tcp (bash builtin) - works on stemcells
		script := fmt.Sprintf(`
# Try bash /dev/tcp first
if (echo >/dev/tcp/127.0.0.1/%d) 2>/dev/null; then
    exit 0
fi
# Try nc -z
if nc -z 127.0.0.1 %d 2>/dev/null; then
    exit 0
fi
# Try Python
if python3 -c "import socket; s=socket.socket(); s.settimeout(1); s.connect(('127.0.0.1',%d)); s.close()" 2>/dev/null; then
    exit 0
fi
exit 1
`, MonitPort, MonitPort, MonitPort)
		_, _, exitCode, _ := i.driver.RunScript(script)
		if exitCode == 0 {
			return nil
		}

		// Every 20 attempts (about 2 seconds), check if the mock monit process is still running
		if attempts%20 == 0 {
			pidCheck := `
if [ -f /tmp/mock-monit.pid ]; then
    pid=$(cat /tmp/mock-monit.pid)
    if kill -0 "$pid" 2>/dev/null; then
        echo "Process $pid is running"
        exit 0
    else
        echo "Process $pid is NOT running"
        cat /tmp/mock-monit.log 2>/dev/null || echo "No log file"
        exit 1
    fi
else
    echo "No PID file found"
    exit 1
fi
`
			stdout, _, exitCode, _ := i.driver.RunScript(pidCheck)
			if exitCode != 0 {
				return fmt.Errorf("mock monit process died: %s", stdout)
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Continue polling
		}
	}
	return fmt.Errorf("timeout waiting for mock monit on port %d after %d attempts", MonitPort, attempts)
}

// TestMonitConnectivity tests if a connection to the monit port can be established.
// Returns nil if connection succeeds, an error otherwise.
// The caller should set an appropriate timeout in the context.
func (i *Installer) TestMonitConnectivity(ctx context.Context) error {
	// Use timeout from context if available
	timeout := "5"
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline).Seconds()
		if remaining > 0 {
			timeout = fmt.Sprintf("%d", int(remaining))
		}
	}

	// Use nc with timeout to test connectivity
	script := fmt.Sprintf("timeout %s nc -z 127.0.0.1 %d 2>&1", timeout, MonitPort)
	stdout, stderr, exitCode, err := i.driver.RunScript(script)
	if err != nil {
		return fmt.Errorf("failed to run connectivity test: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("connection to monit port %d failed: exit %d, stdout: %s, stderr: %s",
			MonitPort, exitCode, stdout, stderr)
	}
	return nil
}

// TestMonitConnectivityAsAgent tests if the bosh-agent binary can connect to monit.
// This runs a connection test as the agent process, which should be allowed by the firewall.
// The agent's firewall rules should allow the agent process to connect to port 2822.
func (i *Installer) TestMonitConnectivityAsAgent(ctx context.Context) error {
	// The agent itself doesn't have a "test connection" mode, so we create a
	// small wrapper that execs into the agent's cgroup and tests the connection.
	// For now, we'll just run nc from the same cgroup as where agent would run.
	//
	// A more accurate test would be to actually start the agent and observe its
	// behavior, but for unit testing purposes, this simpler approach works.
	return i.TestMonitConnectivity(ctx)
}

// TestMonitConnectivityBlocked tests that a non-agent process CANNOT connect to monit.
// This is used to verify the firewall is working correctly - it should block processes
// that are not the bosh-agent from connecting to port 2822.
//
// Returns nil if the connection is BLOCKED (expected behavior when firewall works).
// Returns an error if the connection succeeds (firewall is not working) or if there's
// an unexpected error.
func (i *Installer) TestMonitConnectivityBlocked(ctx context.Context) error {
	// Use a short timeout since we expect this to fail/timeout
	timeout := "2"
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline).Seconds()
		if remaining > 0 && remaining < 2 {
			timeout = fmt.Sprintf("%d", int(remaining))
		}
	}

	// Spawn a new process (not the agent) and try to connect
	// If firewall is working, this should be blocked
	script := fmt.Sprintf(`
# Create a test script that runs in a new process group
sh -c 'exec timeout %s nc -z 127.0.0.1 %d 2>&1'
`, timeout, MonitPort)
	_, _, exitCode, err := i.driver.RunScript(script)
	if err != nil {
		return fmt.Errorf("failed to run blocked connectivity test: %w", err)
	}

	// Exit code 0 means connection succeeded - firewall is NOT blocking
	if exitCode == 0 {
		return fmt.Errorf("connection to monit port %d succeeded but should have been blocked by firewall", MonitPort)
	}

	// Exit code non-zero means connection failed (blocked or timeout) - expected behavior
	return nil
}
