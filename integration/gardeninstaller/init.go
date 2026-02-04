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
// 2. Initialize unprivileged store with XFS loopback (unless SkipUnprivilegedStore is set)
// 3. Initialize privileged store with XFS loopback
//
// When UseDirectStore is true, we skip XFS loopback creation and just create the
// store directories. This is required for nested containers where loop devices
// are not available. Grootfs will use overlay on the underlying filesystem directly.
// Note: This mode does not support disk quotas.
func (i *Installer) initGrootfsStores() error {
	envsScript := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "envs")
	greenskeeperBin := filepath.Join(i.cfg.BaseDir, "packages", "greenskeeper", "bin", "greenskeeper")

	// Run greenskeeper to set up directories
	i.log("Running greenskeeper to set up directories...")
	// Use "." instead of "source" for POSIX compatibility (dash vs bash)
	greenskeeperScript := fmt.Sprintf(". %s && %s", envsScript, greenskeeperBin)
	stdout, stderr, exitCode, err := i.driver.RunScript(greenskeeperScript)
	if err != nil {
		return fmt.Errorf("greenskeeper failed: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("greenskeeper failed (exit %d): stdout=%s, stderr=%s",
			exitCode, stdout, stderr)
	}
	i.log("Directories set up successfully")

	// When UseDirectStore is true, just create store directories without XFS loopback.
	// This is required for nested containers where /dev/loop* devices are not available.
	if i.cfg.UseDirectStore {
		return i.initDirectStores()
	}

	// Standard initialization with XFS loopback
	return i.initXFSStores()
}

// initDirectStores initializes stores using grootfs init-store without --store-size-bytes.
// When store-size-bytes is not specified (defaults to 0), grootfs skips the XFS loopback
// creation and just creates the whiteout device and directory structure needed for overlay.
// This works in containers where loop devices are not available.
func (i *Installer) initDirectStores() error {
	i.log("Using direct store mode (no XFS loopback, no quotas)...")

	envsScript := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "envs")
	grootfsBin := filepath.Join(i.cfg.BaseDir, "packages", "grootfs", "bin", "grootfs")
	privConfig := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "config", "privileged_grootfs_config.yml")

	// Ensure the parent directory for the store exists before calling init-store.
	// grootfs checks the filesystem type of the store path's parent to validate it's suitable.
	privStoreParent := filepath.Join(i.cfg.BaseDir, "data", "grootfs", "store")
	if err := i.driver.MkdirAll(privStoreParent, 0755); err != nil {
		return fmt.Errorf("failed to create store parent directory %s: %w", privStoreParent, err)
	}
	i.log("Created store parent directory: %s", privStoreParent)

	// Initialize privileged store without store-size-bytes (no XFS loopback)
	i.log("Initializing privileged grootfs store (direct mode, no loopback)...")
	initPrivScript := fmt.Sprintf(`
set -e
. %s
%s --config %s init-store
`, envsScript, grootfsBin, privConfig)

	stdout, stderr, exitCode, err := i.driver.RunScript(initPrivScript)
	if err != nil {
		return fmt.Errorf("privileged store init (direct) failed: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("privileged store init (direct) failed (exit %d): stdout=%s, stderr=%s",
			exitCode, stdout, stderr)
	}
	i.log("Privileged store initialized successfully (direct mode)")

	// Initialize unprivileged store if not skipped
	if !i.cfg.SkipUnprivilegedStore {
		unprivConfig := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "config", "grootfs_config.yml")
		maximusBin := filepath.Join(i.cfg.BaseDir, "packages", "garden-idmapper", "bin", "maximus")

		i.log("Initializing unprivileged grootfs store (direct mode, no loopback)...")
		initUnprivScript := fmt.Sprintf(`
set -e
. %s
maximus=$(%s)
%s --config %s init-store \
  --uid-mapping "0:${maximus}:1" \
  --uid-mapping "1:1:$((maximus-1))" \
  --gid-mapping "0:${maximus}:1" \
  --gid-mapping "1:1:$((maximus-1))"
`, envsScript, maximusBin, grootfsBin, unprivConfig)

		stdout, stderr, exitCode, err = i.driver.RunScript(initUnprivScript)
		if err != nil {
			return fmt.Errorf("unprivileged store init (direct) failed: %w", err)
		}
		if exitCode != 0 {
			return fmt.Errorf("unprivileged store init (direct) failed (exit %d): stdout=%s, stderr=%s",
				exitCode, stdout, stderr)
		}
		i.log("Unprivileged store initialized successfully (direct mode)")
	} else {
		i.log("Skipping unprivileged store (SkipUnprivilegedStore=true)")
	}

	i.log("Direct stores initialized successfully (no quotas)")
	return nil
}

// initXFSStores initializes the stores with XFS loopback for quota support.
// This is the standard mode used on VMs with access to loop devices.
func (i *Installer) initXFSStores() error {
	envsScript := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "envs")
	grootfsBin := filepath.Join(i.cfg.BaseDir, "packages", "grootfs", "bin", "grootfs")
	unprivConfig := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "config", "grootfs_config.yml")
	privConfig := filepath.Join(i.cfg.BaseDir, "jobs", "garden", "config", "privileged_grootfs_config.yml")
	maximusBin := filepath.Join(i.cfg.BaseDir, "packages", "garden-idmapper", "bin", "maximus")

	// Initialize unprivileged store with XFS loopback and uid/gid mappings
	// Skip this for nested containers where loop devices may not work
	if i.cfg.SkipUnprivilegedStore {
		i.log("Skipping unprivileged grootfs store (SkipUnprivilegedStore=true)")
	} else {
		i.log("Initializing unprivileged grootfs store with XFS loopback...")
		initUnprivScript := fmt.Sprintf(`
set -e
. %s
maximus=$(%s)
# Create XFS-backed store with uid/gid mappings
%s --config %s init-store \
  --store-size-bytes %d \
  --uid-mapping "0:${maximus}:1" \
  --uid-mapping "1:1:$((maximus-1))" \
  --gid-mapping "0:${maximus}:1" \
  --gid-mapping "1:1:$((maximus-1))"
`, envsScript, maximusBin, grootfsBin, unprivConfig, defaultStoreSizeBytes)

		stdout, stderr, exitCode, err := i.driver.RunScript(initUnprivScript)
		if err != nil {
			return fmt.Errorf("unprivileged store init failed: %w", err)
		}
		if exitCode != 0 {
			return fmt.Errorf("unprivileged store init failed (exit %d): stdout=%s, stderr=%s",
				exitCode, stdout, stderr)
		}
		i.log("Unprivileged store initialized successfully")
	}

	// Initialize privileged store with XFS loopback (no uid/gid mappings needed)
	i.log("Initializing privileged grootfs store with XFS loopback...")
	initPrivScript := fmt.Sprintf(`
set -e
. %s
%s --config %s init-store --store-size-bytes %d
`, envsScript, grootfsBin, privConfig, defaultStoreSizeBytes)

	stdout, stderr, exitCode, err := i.driver.RunScript(initPrivScript)
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
