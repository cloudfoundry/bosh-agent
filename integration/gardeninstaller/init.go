package gardeninstaller

import (
	"fmt"
	"path/filepath"
)

// Default store size: 10GB (sufficient for multiple stemcell images)
// Each stemcell image expands to ~3GB, so we need 10GB to hold 2+ images
const defaultStoreSizeBytes = 10 * 1024 * 1024 * 1024 // 10GB

// initGrootfsStores initializes the GrootFS stores on the target.
// This follows the garden-runc-release startup sequence:
// 1. Run greenskeeper to set up directories
// 2. Initialize unprivileged store with XFS loopback
// 3. Initialize privileged store with XFS loopback
func (i *Installer) initGrootfsStores() error {
	envsScript := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "envs")
	greenskeeperBin := filepath.Join(i.cfg.BaseDir, "packages", "greenskeeper", "bin", "greenskeeper")
	grootfsBin := filepath.Join(i.cfg.BaseDir, "packages", "grootfs", "bin", "grootfs")
	unprivConfig := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "config", "grootfs_config.yml")
	privConfig := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "config", "privileged_grootfs_config.yml")
	maximusBin := filepath.Join(i.cfg.BaseDir, "packages", "garden-idmapper", "bin", "maximus")

	// Run greenskeeper to set up directories
	i.log("Running greenskeeper to set up directories...")
	greenskeeperScript := fmt.Sprintf("source %s && %s", envsScript, greenskeeperBin)
	stdout, stderr, exitCode, err := i.driver.RunScript(greenskeeperScript)
	if err != nil {
		return fmt.Errorf("greenskeeper failed: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("greenskeeper failed (exit %d): stdout=%s, stderr=%s",
			exitCode, stdout, stderr)
	}
	i.log("Directories set up successfully")

	// Initialize unprivileged store with XFS loopback and uid/gid mappings
	i.log("Initializing unprivileged grootfs store with XFS loopback...")
	initUnprivScript := fmt.Sprintf(`
set -e
source %s
maximus=$(%s)
# Create XFS-backed store with uid/gid mappings
%s --config %s init-store \
  --store-size-bytes %d \
  --uid-mapping "0:${maximus}:1" \
  --uid-mapping "1:1:$((maximus-1))" \
  --gid-mapping "0:${maximus}:1" \
  --gid-mapping "1:1:$((maximus-1))"
`, envsScript, maximusBin, grootfsBin, unprivConfig, defaultStoreSizeBytes)

	stdout, stderr, exitCode, err = i.driver.RunScript(initUnprivScript)
	if err != nil {
		return fmt.Errorf("unprivileged store init failed: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("unprivileged store init failed (exit %d): stdout=%s, stderr=%s",
			exitCode, stdout, stderr)
	}
	i.log("Unprivileged store initialized successfully")

	// Initialize privileged store with XFS loopback (no uid/gid mappings needed)
	i.log("Initializing privileged grootfs store with XFS loopback...")
	initPrivScript := fmt.Sprintf(`
set -e
source %s
%s --config %s init-store --store-size-bytes %d
`, envsScript, grootfsBin, privConfig, defaultStoreSizeBytes)

	stdout, stderr, exitCode, err = i.driver.RunScript(initPrivScript)
	if err != nil {
		return fmt.Errorf("privileged store init failed: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("privileged store init failed (exit %d): stdout=%s, stderr=%s",
			exitCode, stdout, stderr)
	}
	i.log("Privileged store initialized successfully")

	i.log("GrootFS stores initialized successfully")
	return nil
}
