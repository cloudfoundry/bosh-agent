package installerdriver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"

	"code.cloudfoundry.org/garden"
)

// NetInRule specifies a port forwarding rule for containers.
type NetInRule struct {
	HostPort      uint32
	ContainerPort uint32
}

// GardenDriverConfig holds configuration for GardenDriver.
type GardenDriverConfig struct {
	// GardenClient is the Garden API client used to create the container.
	GardenClient garden.Client

	// ParentDriver is the driver for the parent environment (used to create
	// the host-side bind mount directory). For L1 containers, this is the
	// SSHDriver to the host VM. For L2 containers, this is the L1 GardenDriver.
	ParentDriver Driver

	// Handle is the container handle (unique identifier).
	Handle string

	// Image is the OCI image URI. If empty, uses Garden's default rootfs.
	Image string

	// Network specifies the container's network configuration in CIDR notation.
	// Format: "a.b.c.d/n" where a.b.c.d is the desired IP and n is the prefix length.
	// Example: "10.254.0.10/22" assigns IP 10.254.0.10 from the 10.254.0.0/22 subnet.
	// If empty, Garden allocates an IP from its default pool.
	Network string

	// NetIn specifies port forwarding rules.
	NetIn []NetInRule

	// DiskLimit is the disk limit in bytes. 0 means no limit.
	DiskLimit uint64

	// SkipCgroupMount when true, does not bind-mount /sys/fs/cgroup into the container.
	// This simulates warden-cpi's default behavior (without systemd mode) where
	// containers don't have access to the cgroup filesystem. Used to test the
	// bosh-agent's firewall behavior when cgroup detection fails.
	SkipCgroupMount bool

	// UseSystemd when true, starts systemd as PID 1 in the container.
	// This requires:
	// 1. /sys/fs/cgroup to be bind-mounted (SkipCgroupMount must be false)
	// 2. The stemcell image to have systemd installed
	//
	// When UseSystemd is true:
	// - The bosh-agent.service is disabled before systemd starts
	// - systemd runs as PID 1, managing the container's processes
	// - Processes run in proper cgroup slices (e.g., /system.slice/foo.service)
	//
	// This enables testing of cgroup-based firewall isolation, where the agent
	// runs in a different cgroup than other processes.
	UseSystemd bool
}

// GardenDriver implements Driver for Garden containers.
// It creates and manages a container during Bootstrap().
type GardenDriver struct {
	// Config (set at construction)
	gardenClient    garden.Client
	parentDriver    Driver
	handle          string
	image           string
	network         string
	netIn           []NetInRule
	diskLimit       uint64
	skipCgroupMount bool
	useSystemd      bool

	// State (set by Bootstrap)
	container    garden.Container
	hostDataDir  string
	bootstrapped bool
}

// NewGardenDriver creates a new driver with the given configuration.
// The container is not created until Bootstrap() is called.
func NewGardenDriver(cfg GardenDriverConfig) *GardenDriver {
	return &GardenDriver{
		gardenClient:    cfg.GardenClient,
		parentDriver:    cfg.ParentDriver,
		handle:          cfg.Handle,
		image:           cfg.Image,
		network:         cfg.Network,
		netIn:           cfg.NetIn,
		diskLimit:       cfg.DiskLimit,
		skipCgroupMount: cfg.SkipCgroupMount,
		useSystemd:      cfg.UseSystemd,
	}
}

// Description returns a human-readable description of the target.
func (d *GardenDriver) Description() string {
	return fmt.Sprintf("garden-container:%s", d.handle)
}

// ContainerIP returns the IP address of the container.
// This can be used to connect to services running inside the container directly,
// bypassing NetIn port forwarding which may not work in all environments.
func (d *GardenDriver) ContainerIP() (string, error) {
	if err := d.checkBootstrapped(); err != nil {
		return "", err
	}
	info, err := d.container.Info()
	if err != nil {
		return "", fmt.Errorf("failed to get container info: %w", err)
	}
	if info.ContainerIP == "" {
		return "", fmt.Errorf("container has no IP address")
	}
	return info.ContainerIP, nil
}

// Handle returns the container handle.
func (d *GardenDriver) Handle() string {
	return d.handle
}

// Container returns the underlying Garden container.
// This can be used for advanced operations like running processes or tunneling traffic.
// Returns nil if Bootstrap() hasn't been called.
func (d *GardenDriver) Container() garden.Container {
	return d.container
}

// IsBootstrapped returns true if Bootstrap() has been called successfully.
func (d *GardenDriver) IsBootstrapped() bool {
	return d.bootstrapped
}

// Bootstrap creates the container and prepares it for use.
// This includes:
// 1. Creating the host-side bind mount directory via parentDriver
// 2. Creating the container with bind mounts for cgroup, lib/modules, and data
// 3. Setting up port forwarding
// 4. Unmounting Garden's bind-mounted files and configuring DNS
func (d *GardenDriver) Bootstrap() error {
	// 1. Create host-side bind mount directory via parentDriver
	d.hostDataDir = filepath.Join(BaseDir, "data", "garden-containers", d.handle)
	if err := d.parentDriver.MkdirAll(d.hostDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create host data directory %s: %w", d.hostDataDir, err)
	}

	// Verify directory was created and check mount info
	stdout, stderr, exitCode, err := d.parentDriver.RunCommand("sh", "-c",
		fmt.Sprintf("ls -la %s && cat /proc/self/mountinfo | grep -E '(cgroup|/var/vcap)' | head -20", d.hostDataDir))
	if err != nil || exitCode != 0 {
		fmt.Printf("[GardenDriver.Bootstrap] Warning: failed to verify directory %s: err=%v, exit=%d, stdout=%s, stderr=%s\n",
			d.hostDataDir, err, exitCode, stdout, stderr)
	} else {
		fmt.Printf("[GardenDriver.Bootstrap] Directory verified: %s\n%s\n", d.hostDataDir, stdout)
	}

	// 2. Build container spec with standard bind mounts
	bindMounts := []garden.BindMount{}

	// Conditionally add cgroup bind mount
	if !d.skipCgroupMount {
		bindMounts = append(bindMounts, garden.BindMount{
			SrcPath: "/sys/fs/cgroup",
			DstPath: "/sys/fs/cgroup",
			Mode:    garden.BindMountModeRW,
			Origin:  garden.BindMountOriginHost,
		})
	}

	// Always add lib/modules and data mounts
	bindMounts = append(bindMounts,
		garden.BindMount{
			SrcPath: "/lib/modules",
			DstPath: "/lib/modules",
			Mode:    garden.BindMountModeRO,
			Origin:  garden.BindMountOriginHost,
		},
		garden.BindMount{
			// Bind mount host directory to /var/vcap/data in container.
			// This provides access to the host's data disk for packages,
			// Garden depot, and GrootFS store.
			SrcPath: d.hostDataDir,
			DstPath: filepath.Join(BaseDir, "data"),
			Mode:    garden.BindMountModeRW,
			Origin:  garden.BindMountOriginHost,
		},
	)

	spec := garden.ContainerSpec{
		Handle:     d.handle,
		Privileged: true,
		Properties: garden.Properties{
			"installerdriver": "true",
		},
		BindMounts: bindMounts,
	}

	// Set image if specified
	if d.image != "" {
		spec.Image = garden.ImageRef{URI: d.image}
	}

	// Set network/static IP if specified
	if d.network != "" {
		spec.Network = d.network
	}

	// Set disk limit if specified
	if d.diskLimit > 0 {
		spec.Limits = garden.Limits{
			Disk: garden.DiskLimits{
				ByteHard: d.diskLimit,
			},
		}
	}

	// 3. Create container
	container, err := d.gardenClient.Create(spec)
	if err != nil {
		// Cleanup host directory on failure
		_, _, _, _ = d.parentDriver.RunCommand("rm", "-rf", d.hostDataDir)
		d.hostDataDir = ""
		return fmt.Errorf("failed to create container: %w", err)
	}
	d.container = container

	// 4. Set up port forwarding
	for _, rule := range d.netIn {
		if rule.HostPort > 0 && rule.ContainerPort > 0 {
			_, _, err := container.NetIn(rule.HostPort, rule.ContainerPort)
			if err != nil {
				// Cleanup on failure
				_ = d.cleanupContainer()
				return fmt.Errorf("failed to set up port forwarding %d->%d: %w",
					rule.HostPort, rule.ContainerPort, err)
			}
		}
	}

	// 5. Container initialization depends on mode
	if d.useSystemd {
		// Systemd mode: disable bosh-agent services and start systemd as PID 1
		if err := d.bootstrapSystemd(); err != nil {
			_ = d.cleanupContainer()
			return err
		}
	} else {
		// Non-systemd mode: just unmount bind-mounted files and configure DNS
		if err := d.bootstrapNonSystemd(); err != nil {
			_ = d.cleanupContainer()
			return err
		}
	}

	d.bootstrapped = true
	return nil
}

// bootstrapNonSystemd prepares the container for non-systemd operation.
// This unmounts Garden's bind-mounted files and configures DNS.
func (d *GardenDriver) bootstrapNonSystemd() error {
	unmountScript := `
umount /etc/resolv.conf 2>/dev/null || true
umount /etc/hosts 2>/dev/null || true
umount /etc/hostname 2>/dev/null || true

# Configure DNS with Google's public DNS servers
cat > /etc/resolv.conf <<EOF
nameserver 8.8.8.8
nameserver 8.8.4.4
EOF
`
	_, stderr, exitCode, err := d.runScriptInternal(unmountScript)
	if err != nil {
		return fmt.Errorf("failed to unmount bind-mounted files: %w", err)
	}
	if exitCode != 0 {
		// Log but don't fail - unmount may fail if files weren't mounted
		_ = stderr // suppress unused warning
	}
	return nil
}

// bootstrapSystemd prepares the container for systemd operation.
// This:
// 1. Disables bosh-agent.service so our test agent can be installed separately
// 2. Unmounts Garden's bind-mounted files
// 3. Starts systemd as PID 1 in the background
func (d *GardenDriver) bootstrapSystemd() error {
	if d.skipCgroupMount {
		return fmt.Errorf("UseSystemd requires cgroup mount - SkipCgroupMount must be false")
	}

	// Disable bosh-agent services so our test can install and start its own agent.
	// Also unmount Garden's bind-mounted files and configure DNS.
	prepareScript := `
set -e

# Unmount Garden's bind-mounted files
umount /etc/resolv.conf 2>/dev/null || true
umount /etc/hosts 2>/dev/null || true
umount /etc/hostname 2>/dev/null || true

# Configure DNS
cat > /etc/resolv.conf <<EOF
nameserver 8.8.8.8
nameserver 8.8.4.4
EOF

# Disable bosh-agent services so systemd won't start them automatically.
# This allows our test installer to install and start the agent manually.
systemctl mask bosh-agent.service 2>/dev/null || true
systemctl mask bosh-agent-wait.service 2>/dev/null || true

# Also disable via runsvdir in case the stemcell uses runit
rm -rf /etc/sv/bosh-agent 2>/dev/null || true
rm -rf /etc/service/bosh-agent 2>/dev/null || true

echo "Container prepared for systemd"
`
	stdout, stderr, exitCode, err := d.runScriptInternal(prepareScript)
	if err != nil {
		return fmt.Errorf("failed to prepare container for systemd: %w (stdout: %s, stderr: %s)", err, stdout, stderr)
	}
	if exitCode != 0 {
		return fmt.Errorf("failed to prepare container for systemd: exit %d (stdout: %s, stderr: %s)", exitCode, stdout, stderr)
	}

	// Start systemd as PID 1.
	// We use 'exec /sbin/init' to replace the current process with systemd.
	// This runs in the background (no Wait) so the container continues running.
	//
	// Note: In Garden containers, processes run via container.Run() are not PID 1.
	// However, starting /sbin/init (which is symlinked to systemd on Noble stemcells)
	// will initialize systemd and allow it to manage services.
	//
	// We run systemd with --system to ensure it runs in system mode.
	systemdStartScript := `
# Start systemd in the background
# Using nohup to ensure it survives the parent shell exiting
nohup /sbin/init --system < /dev/null > /var/log/systemd-init.log 2>&1 &
echo "systemd starting as PID $!"

# Wait a moment for systemd to initialize
sleep 2

# Verify systemd is running
if systemctl is-system-running --wait 2>/dev/null; then
    echo "systemd is running"
elif systemctl is-system-running 2>/dev/null | grep -qE '(running|starting|initializing|degraded)'; then
    echo "systemd state: $(systemctl is-system-running)"
else
    echo "Warning: systemd may not be fully operational"
    systemctl is-system-running 2>&1 || true
fi
`
	stdout, stderr, exitCode, err = d.runScriptInternal(systemdStartScript)
	if err != nil {
		return fmt.Errorf("failed to start systemd: %w (stdout: %s, stderr: %s)", err, stdout, stderr)
	}
	// Don't fail on non-zero exit code - systemd might report "degraded" state
	// which is acceptable for our testing purposes
	fmt.Printf("[GardenDriver.bootstrapSystemd] systemd start result: exit=%d, stdout=%s, stderr=%s\n",
		exitCode, stdout, stderr)

	return nil
}

// Cleanup destroys the container and removes the host-side bind mount directory.
func (d *GardenDriver) Cleanup() error {
	if err := d.cleanupContainer(); err != nil {
		return err
	}
	d.bootstrapped = false
	return nil
}

// cleanupContainer destroys the container and removes the host data directory.
func (d *GardenDriver) cleanupContainer() error {
	if d.container != nil {
		// Stop container
		_ = d.container.Stop(true)

		// Destroy container
		if err := d.gardenClient.Destroy(d.handle); err != nil {
			return fmt.Errorf("failed to destroy container: %w", err)
		}
		d.container = nil
	}

	if d.hostDataDir != "" {
		_, _, _, err := d.parentDriver.RunCommand("rm", "-rf", d.hostDataDir)
		if err != nil {
			return fmt.Errorf("failed to remove host data directory: %w", err)
		}
		d.hostDataDir = ""
	}

	return nil
}

// checkBootstrapped returns an error if Bootstrap() hasn't been called.
func (d *GardenDriver) checkBootstrapped() error {
	if !d.bootstrapped {
		return ErrNotBootstrapped
	}
	return nil
}

// RunCommand executes a command in the container.
func (d *GardenDriver) RunCommand(path string, args ...string) (stdout, stderr string, exitCode int, err error) {
	if err := d.checkBootstrapped(); err != nil {
		return "", "", -1, err
	}
	return d.runCommandInternal(path, args...)
}

// runCommandInternal executes a command without bootstrap check.
func (d *GardenDriver) runCommandInternal(path string, args ...string) (stdout, stderr string, exitCode int, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer

	processSpec := garden.ProcessSpec{
		Path: path,
		Args: args,
		User: "root",
	}

	processIO := garden.ProcessIO{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
	}

	process, err := d.container.Run(processSpec, processIO)
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to run command: %w", err)
	}

	exitCode, err = process.Wait()
	if err != nil {
		return stdoutBuf.String(), stderrBuf.String(), exitCode, fmt.Errorf("failed waiting for command: %w", err)
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}

// RunScript executes a shell script in the container.
func (d *GardenDriver) RunScript(script string) (stdout, stderr string, exitCode int, err error) {
	if err := d.checkBootstrapped(); err != nil {
		return "", "", -1, err
	}
	return d.runScriptInternal(script)
}

// runScriptInternal executes a shell script without bootstrap check.
func (d *GardenDriver) runScriptInternal(script string) (stdout, stderr string, exitCode int, err error) {
	return d.runCommandInternal("sh", "-c", script)
}

// WriteFile writes content to a file in the container.
func (d *GardenDriver) WriteFile(path string, content []byte, mode int64) error {
	if err := d.checkBootstrapped(); err != nil {
		return err
	}

	// Create tar archive with the file
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	header := &tar.Header{
		Name: tarBaseName(path),
		Mode: mode,
		Size: int64(len(content)),
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("failed to write tar content: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Stream into container at the directory containing the file
	spec := garden.StreamInSpec{
		Path:      tarDirName(path),
		User:      "root",
		TarStream: &buf,
	}

	if err := d.container.StreamIn(spec); err != nil {
		return fmt.Errorf("failed to stream into container: %w", err)
	}

	return nil
}

// ReadFile reads a file from the container.
func (d *GardenDriver) ReadFile(path string) ([]byte, error) {
	if err := d.checkBootstrapped(); err != nil {
		return nil, err
	}

	spec := garden.StreamOutSpec{
		Path: path,
		User: "root",
	}

	reader, err := d.container.StreamOut(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to stream out from container: %w", err)
	}
	defer reader.Close()

	// Read tar archive
	tr := tar.NewReader(reader)

	// Get the first file from the tar
	_, err = tr.Next()
	if err != nil {
		return nil, fmt.Errorf("failed to read tar header: %w", err)
	}

	content, err := io.ReadAll(tr)
	if err != nil {
		return nil, fmt.Errorf("failed to read tar content: %w", err)
	}

	return content, nil
}

// MkdirAll creates a directory and all parent directories.
func (d *GardenDriver) MkdirAll(path string, mode int64) error {
	if err := d.checkBootstrapped(); err != nil {
		return err
	}

	stdout, stderr, exitCode, err := d.runCommandInternal("mkdir", "-p", path)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("mkdir failed with exit code %d: stdout=%s, stderr=%s", exitCode, stdout, stderr)
	}
	return nil
}

// StreamTarball streams a gzipped tarball and extracts it to destDir.
func (d *GardenDriver) StreamTarball(r io.Reader, destDir string) error {
	if err := d.checkBootstrapped(); err != nil {
		return err
	}

	// Read the tarball data
	compressedData, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read tarball data: %w", err)
	}

	// Garden's StreamIn expects an uncompressed tar, so we need to decompress
	gr, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	spec := garden.StreamInSpec{
		Path:      destDir,
		User:      "root",
		TarStream: gr,
	}

	if err := d.container.StreamIn(spec); err != nil {
		return fmt.Errorf("failed to stream tarball into container: %w", err)
	}

	return nil
}

// Chmod changes the file mode of the specified path.
func (d *GardenDriver) Chmod(path string, mode int64) error {
	if err := d.checkBootstrapped(); err != nil {
		return err
	}

	modeStr := fmt.Sprintf("%o", mode)
	stdout, stderr, exitCode, err := d.runCommandInternal("chmod", modeStr, path)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("chmod failed with exit code %d: stdout=%s, stderr=%s", exitCode, stdout, stderr)
	}
	return nil
}

// tarBaseName returns the base name of a path for tar headers.
func tarBaseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

// tarDirName returns the directory portion of a path for tar streaming.
func tarDirName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "/"
}

// Verify GardenDriver implements Driver
var _ Driver = (*GardenDriver)(nil)
