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

	// NetIn specifies port forwarding rules.
	NetIn []NetInRule

	// DiskLimit is the disk limit in bytes. 0 means no limit.
	DiskLimit uint64
}

// GardenDriver implements Driver for Garden containers.
// It creates and manages a container during Bootstrap().
type GardenDriver struct {
	// Config (set at construction)
	gardenClient garden.Client
	parentDriver Driver
	handle       string
	image        string
	netIn        []NetInRule
	diskLimit    uint64

	// State (set by Bootstrap)
	container    garden.Container
	hostDataDir  string
	bootstrapped bool
}

// NewGardenDriver creates a new driver with the given configuration.
// The container is not created until Bootstrap() is called.
func NewGardenDriver(cfg GardenDriverConfig) *GardenDriver {
	return &GardenDriver{
		gardenClient: cfg.GardenClient,
		parentDriver: cfg.ParentDriver,
		handle:       cfg.Handle,
		image:        cfg.Image,
		netIn:        cfg.NetIn,
		diskLimit:    cfg.DiskLimit,
	}
}

// Description returns a human-readable description of the target.
func (d *GardenDriver) Description() string {
	return fmt.Sprintf("garden-container:%s", d.handle)
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

	// 2. Build container spec with standard bind mounts
	spec := garden.ContainerSpec{
		Handle:     d.handle,
		Privileged: true,
		Properties: garden.Properties{
			"installerdriver": "true",
		},
		// Standard bind mounts for running Garden inside the container
		BindMounts: []garden.BindMount{
			{
				SrcPath: "/sys/fs/cgroup",
				DstPath: "/sys/fs/cgroup",
				Mode:    garden.BindMountModeRW,
				Origin:  garden.BindMountOriginHost,
			},
			{
				SrcPath: "/lib/modules",
				DstPath: "/lib/modules",
				Mode:    garden.BindMountModeRO,
				Origin:  garden.BindMountOriginHost,
			},
			{
				// Bind mount host directory to /var/vcap/data in container.
				// This provides access to the host's data disk for packages,
				// Garden depot, and GrootFS store.
				SrcPath: d.hostDataDir,
				DstPath: filepath.Join(BaseDir, "data"),
				Mode:    garden.BindMountModeRW,
				Origin:  garden.BindMountOriginHost,
			},
		},
	}

	// Set image if specified
	if d.image != "" {
		spec.Image = garden.ImageRef{URI: d.image}
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

	// 5. Unmount bind-mounted files and configure DNS
	// Garden bind-mounts /etc/resolv.conf, /etc/hosts, /etc/hostname from host.
	// We need to unmount them so the container can modify these files.
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
		_ = d.cleanupContainer()
		return fmt.Errorf("failed to unmount bind-mounted files: %w", err)
	}
	if exitCode != 0 {
		// Log but don't fail - unmount may fail if files weren't mounted
		_ = stderr // suppress unused warning
	}

	d.bootstrapped = true
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
