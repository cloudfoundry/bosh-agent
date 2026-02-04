// Package gardeninstaller extracts and installs Garden from a compiled BOSH release tarball.
// It replaces the Ruby install-garden.rb script with pure Go for use in integration tests.
//
// The package uses the installerdriver.Driver interface to abstract the target environment,
// allowing installation to VMs (via SSH) or Garden containers (via Garden API).
package gardeninstaller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/bosh-agent/v2/integration/installerdriver"
)

// Config holds the configuration for installing Garden.
type Config struct {
	// ReleaseTarballPath is the path to the compiled garden-runc release tarball (local path).
	ReleaseTarballPath string

	// BaseDir is the BOSH installation directory on the target (default: /var/vcap).
	BaseDir string

	// ListenNetwork is the network type for the Garden server (default: tcp).
	ListenNetwork string

	// ListenAddress is the address for the Garden server (default: 0.0.0.0:7777).
	ListenAddress string

	// AllowHostAccess allows containers to access the host network (default: true).
	AllowHostAccess bool

	// DestroyOnStart destroys existing containers on Garden startup (default: true).
	DestroyOnStart bool

	// StoreSizeBytes is the size in bytes for the XFS backing stores (default: 10GB).
	// This is passed to grootfs init-store --store-size-bytes.
	// In containers, disk space detection may return very small values, so an explicit
	// size is required to avoid "agsize too small" errors from mkfs.xfs.
	StoreSizeBytes int64

	// Debug enables debug logging during installation.
	Debug bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseDir:         "/var/vcap",
		ListenNetwork:   "tcp",
		ListenAddress:   "0.0.0.0:7777",
		AllowHostAccess: true,
		DestroyOnStart:  true,
		StoreSizeBytes:  10 * 1024 * 1024 * 1024, // 10GB
		Debug:           false,
	}
}

// Installer installs Garden from a compiled BOSH release to a target environment.
type Installer struct {
	cfg    Config
	driver installerdriver.Driver
}

// New creates a new Installer with the given configuration and driver.
func New(cfg Config, driver installerdriver.Driver) *Installer {
	return &Installer{cfg: cfg, driver: driver}
}

// Install extracts packages and generates config from a compiled garden-runc release.
// It performs the following steps:
// 1. Create required directories on target
// 2. Extract compiled packages from the tarball and stream to target
// 3. Render ERB templates and copy static templates using Ruby
//
// Note: This requires Ruby to be installed locally for ERB template rendering.
// Templates are rendered locally and streamed to the target.
// GrootFS store initialization happens during Start() via the garden_start script.
func (i *Installer) Install() error {
	if !i.driver.IsBootstrapped() {
		return fmt.Errorf("driver not bootstrapped: call driver.Bootstrap() before installer.Install()")
	}

	if i.cfg.ReleaseTarballPath == "" {
		return fmt.Errorf("release tarball path is required")
	}

	if _, err := os.Stat(i.cfg.ReleaseTarballPath); err != nil {
		return fmt.Errorf("release tarball not found: %w", err)
	}

	i.log("Installing Garden from %s to %s", i.cfg.ReleaseTarballPath, i.driver.Description())

	// Step 1: Create directories on target
	if err := i.createDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Step 2: Extract packages locally and stream to target
	packages, err := i.extractPackages()
	if err != nil {
		return fmt.Errorf("failed to extract packages: %w", err)
	}
	i.log("Extracted %d packages", len(packages))

	// Step 3: Render ERB templates and copy static templates
	// This extracts templates from the release tarball, renders ERB using Ruby,
	// and streams all configs to the target
	if err := i.generateConfigs(); err != nil {
		return fmt.Errorf("failed to generate configs: %w", err)
	}

	i.log("Garden installation complete on %s", i.driver.Description())
	return nil
}

// Start starts the Garden server on the target.
// This follows the BOSH job lifecycle:
// 1. Run pre-start script (permit_device_control, invoke_thresholder)
// 2. Run garden_start in background (loop devices, XFS setup, grootfs init, containerd, gdn)
// 3. Wait for Garden API to become available
func (i *Installer) Start() error {
	// Step 1: Run pre-start script (BOSH lifecycle - runs before monit start)
	preStart := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "pre-start")
	i.log("Running pre-start script...")
	stdout, stderr, exitCode, err := i.driver.RunCommand(preStart)
	if err != nil {
		return fmt.Errorf("failed to run pre-start: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("pre-start failed with exit code %d: stdout=%s, stderr=%s", exitCode, stdout, stderr)
	}
	i.log("pre-start completed")

	// Step 2: Run garden_start in background (monit start command)
	// garden_start handles: greenskeeper, create_loop_devices, permit_device_control,
	// overlay-xfs-setup (XFS loopback + grootfs init), containerd, gdn setup, gdn server
	// The script runs gdn server with exec, so it blocks forever - we must background it.
	gardenStart := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "garden_start")
	i.log("Starting Garden in background...")
	startScript := fmt.Sprintf("nohup %s > /var/vcap/sys/log/garden/garden_start.log 2>&1 &", gardenStart)
	stdout, stderr, exitCode, err = i.driver.RunScript(startScript)
	if err != nil {
		return fmt.Errorf("failed to start garden: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("garden start failed with exit code %d: stdout=%s, stderr=%s", exitCode, stdout, stderr)
	}

	// Step 3: Wait for Garden API to become available
	if err := i.waitForGarden(); err != nil {
		return fmt.Errorf("garden failed to start: %w", err)
	}

	i.log("Garden started on %s", i.driver.Description())
	return nil
}

// waitForGarden polls the Garden API until it responds or times out.
func (i *Installer) waitForGarden() error {
	// Parse listen address to determine how to check
	address := i.cfg.ListenAddress
	if address == "" {
		address = "0.0.0.0:7777"
	}

	// Extract port from address (format: "host:port")
	parts := strings.Split(address, ":")
	port := "7777"
	if len(parts) >= 2 {
		port = parts[len(parts)-1]
	}

	// Poll using nc (netcat) or bash to check if the port is listening
	// Use /dev/tcp if nc is not available (works in bash)
	checkScript := fmt.Sprintf(`
for i in $(seq 1 60); do
    if command -v nc >/dev/null 2>&1; then
        if nc -z 127.0.0.1 %s 2>/dev/null; then
            echo "Garden API ready after ${i}s"
            exit 0
        fi
    else
        if (echo > /dev/tcp/127.0.0.1/%s) 2>/dev/null; then
            echo "Garden API ready after ${i}s"
            exit 0
        fi
    fi
    sleep 1
done
echo "Timeout waiting for Garden API on port %s"
exit 1
`, port, port, port)

	i.log("Waiting for Garden API on port %s...", port)
	stdout, stderr, exitCode, err := i.driver.RunScript(checkScript)
	if err != nil {
		return fmt.Errorf("failed to check garden status: %w (stdout=%s, stderr=%s)", err, stdout, stderr)
	}
	if exitCode != 0 {
		// Try to get logs for debugging (use tail -n 100 for busybox compatibility)
		logs, _, _, _ := i.driver.RunScript("for f in /var/vcap/sys/log/garden/*.log; do echo '=== '$f' ==='; tail -n 100 $f 2>/dev/null || cat $f 2>/dev/null || echo 'file not found'; done")
		// Also check if any processes are running
		procs, _, _, _ := i.driver.RunScript("ps aux 2>/dev/null | grep -v grep | head -50 || ps 2>/dev/null || echo 'ps not available'")
		return fmt.Errorf("garden did not start within timeout: stdout=%s, stderr=%s, logs=%s, procs=%s", stdout, stderr, logs, procs)
	}
	i.log("Garden API ready: %s", strings.TrimSpace(stdout))
	return nil
}

// Stop stops the Garden server on the target.
func (i *Installer) Stop() error {
	gardenStop := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "garden_stop")
	stdout, stderr, exitCode, err := i.driver.RunCommand(gardenStop)
	if err != nil {
		return fmt.Errorf("failed to stop garden: %w", err)
	}
	if exitCode != 0 {
		// Non-zero exit might be ok if garden wasn't running
		i.log("garden stop returned exit code %d: stdout=%s, stderr=%s", exitCode, stdout, stderr)
	}
	i.log("Garden stopped on %s", i.driver.Description())
	return nil
}

// createDirectories creates the required directory structure on the target.
func (i *Installer) createDirectories() error {
	// When running in nested containers with bind-mounted /var/vcap/data (from host),
	// we have access to a large disk. The root overlay filesystem is very limited (~10GB)
	// and gets exhausted by the stemcell content (~9.5GB).
	//
	// We symlink /var/vcap/packages to /var/vcap/data/packages to ensure package
	// extraction uses the bind-mounted disk instead of the overlay.
	//
	// This follows the bosh-warden-cpi pattern where /var/vcap/data is bind-mounted
	// from the host, providing access to the host's data disk (100+ GB).

	// First, create /var/vcap/data/packages (on bind-mounted disk)
	packagesOnData := filepath.Join(i.cfg.BaseDir, "data", "packages")
	i.log("Creating packages directory on data disk: %s", packagesOnData)
	if err := i.driver.MkdirAll(packagesOnData, 0755); err != nil {
		return fmt.Errorf("failed to create packages directory on data: %w", err)
	}

	// Ensure base dir exists for the symlink
	if err := i.driver.MkdirAll(i.cfg.BaseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base dir: %w", err)
	}

	// Create symlink: /var/vcap/packages -> /var/vcap/data/packages
	packagesPath := filepath.Join(i.cfg.BaseDir, "packages")
	i.log("Creating symlink: %s -> %s", packagesPath, packagesOnData)
	linkScript := fmt.Sprintf("rm -rf %s && ln -sf %s %s", packagesPath, packagesOnData, packagesPath)
	stdout, stderr, exitCode, err := i.driver.RunScript(linkScript)
	if err != nil {
		return fmt.Errorf("failed to create packages symlink: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("packages symlink failed: exit=%d stdout=%s stderr=%s", exitCode, stdout, stderr)
	}

	// Now create remaining directories
	dirs := []string{
		filepath.Join(i.cfg.BaseDir, "sys", "run", "garden"),
		filepath.Join(i.cfg.BaseDir, "sys", "log", "garden"),
		filepath.Join(i.cfg.BaseDir, "data", "garden", "bin"),
		filepath.Join(i.cfg.BaseDir, "data", "garden", "depot"), // depot is on /var/vcap/data (bind-mounted)
		filepath.Join(i.cfg.BaseDir, "data", "tmp"),
		filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin"),
		filepath.Join(i.cfg.BaseDir, "jobs", "garden", "config"),
		// packages is handled above with symlink to /var/vcap/data/packages
	}

	for _, dir := range dirs {
		i.log("Creating directory: %s", dir)
		if err := i.driver.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	return nil
}

func (i *Installer) log(format string, args ...interface{}) {
	if i.cfg.Debug {
		fmt.Printf("[gardeninstaller] "+format+"\n", args...)
	}
}
