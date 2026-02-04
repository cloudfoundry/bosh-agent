// Package gardeninstaller extracts and installs Garden from a compiled BOSH release tarball.
// It replaces the Ruby install-garden.rb script with pure Go for use in integration tests.
//
// The package uses a Driver interface to abstract the target environment, allowing
// installation to VMs (via SSH) or Garden containers (via Garden API).
package gardeninstaller

import (
	"fmt"
	"os"
	"path/filepath"
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

	// SkipUnprivilegedStore skips initialization of the unprivileged GrootFS store.
	// Set to true for nested Garden installations where loop devices may not work.
	SkipUnprivilegedStore bool

	// UseDirectStore uses direct overlay storage instead of XFS loopback.
	// This is required for nested containers where loop devices are not available.
	// With direct store, grootfs uses the host filesystem directly (no quotas).
	UseDirectStore bool

	// Debug enables debug logging during installation.
	Debug bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseDir:               "/var/vcap",
		ListenNetwork:         "tcp",
		ListenAddress:         "0.0.0.0:7777",
		AllowHostAccess:       true,
		DestroyOnStart:        true,
		SkipUnprivilegedStore: false,
		Debug:                 false,
	}
}

// Installer installs Garden from a compiled BOSH release to a target environment.
type Installer struct {
	cfg    Config
	driver Driver
}

// New creates a new Installer with the given configuration and driver.
func New(cfg Config, driver Driver) *Installer {
	return &Installer{cfg: cfg, driver: driver}
}

// Install extracts packages and generates config from a compiled garden-runc release.
// It performs the following steps:
// 1. Create required directories on target
// 2. Extract compiled packages from the tarball and stream to target
// 3. Extract non-ERB job templates (overlay-xfs-setup, etc.)
// 4. Generate config files (config.ini, garden_ctl, envs, grootfs-utils) on target
// 5. Initialize GrootFS stores on target using overlay-xfs-setup
func (i *Installer) Install() error {
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

	// Step 3: Extract non-ERB job templates
	if err := i.extractJobTemplates(); err != nil {
		return fmt.Errorf("failed to extract job templates: %w", err)
	}

	// Step 4: Generate config files on target
	if err := i.generateConfigs(); err != nil {
		return fmt.Errorf("failed to generate configs: %w", err)
	}

	// Step 5: Initialize GrootFS stores on target using overlay-xfs-setup
	if err := i.initGrootfsStores(); err != nil {
		return fmt.Errorf("failed to initialize grootfs stores: %w", err)
	}

	i.log("Garden installation complete on %s", i.driver.Description())
	return nil
}

// Start starts the Garden server on the target.
func (i *Installer) Start() error {
	gardenCtl := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "garden_ctl")
	stdout, stderr, exitCode, err := i.driver.RunCommand(gardenCtl, "start")
	if err != nil {
		return fmt.Errorf("failed to start garden: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("garden start failed with exit code %d: stdout=%s, stderr=%s", exitCode, stdout, stderr)
	}
	i.log("Garden started on %s", i.driver.Description())
	return nil
}

// Stop stops the Garden server on the target.
func (i *Installer) Stop() error {
	gardenCtl := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "garden_ctl")
	stdout, stderr, exitCode, err := i.driver.RunCommand(gardenCtl, "stop")
	if err != nil {
		return fmt.Errorf("failed to stop garden: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("garden stop failed with exit code %d: stdout=%s, stderr=%s", exitCode, stdout, stderr)
	}
	i.log("Garden stopped on %s", i.driver.Description())
	return nil
}

// createDirectories creates the required directory structure on the target.
func (i *Installer) createDirectories() error {
	dirs := []string{
		filepath.Join(i.cfg.BaseDir, "sys", "run", "garden"),
		filepath.Join(i.cfg.BaseDir, "sys", "log", "garden"),
		filepath.Join(i.cfg.BaseDir, "data", "garden", "depot"),
		filepath.Join(i.cfg.BaseDir, "data", "garden", "bin"),
		filepath.Join(i.cfg.BaseDir, "data", "tmp"),
		filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin"),
		filepath.Join(i.cfg.BaseDir, "jobs", "garden", "config"),
		filepath.Join(i.cfg.BaseDir, "packages"),
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
