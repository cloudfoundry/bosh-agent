package utils

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager/v3"

	"github.com/cloudfoundry/bosh-agent/v2/integration/windows/utils"
)

const (
	// NobleStemcellImage is the OCI image for Ubuntu Noble stemcell
	NobleStemcellImage = "docker://ghcr.io/cloudfoundry/ubuntu-noble-stemcell:latest"
	// JammyStemcellImage is the OCI image for Ubuntu Jammy stemcell
	JammyStemcellImage = "docker://ghcr.io/cloudfoundry/ubuntu-jammy-stemcell:latest"
	// DefaultStemcellImage is the default OCI image to use for creating containers
	DefaultStemcellImage = NobleStemcellImage
)

// GardenClient wraps a Garden client for creating and managing containers
// through an SSH tunnel to a remote Garden daemon.
type GardenClient struct {
	client        garden.Client
	container     garden.Container
	logger        lager.Logger
	stemcellImage string
}

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

// NewGardenClient creates a new GardenClient that connects through an SSH tunnel.
// The SSH tunnel uses the same jumpbox configuration as the NATS client.
// Uses the default stemcell image (or STEMCELL_IMAGE env var if set).
func NewGardenClient() (*GardenClient, error) {
	return NewGardenClientWithImage(StemcellImage())
}

// NewGardenClientWithImage creates a new GardenClient configured to use the specified stemcell image.
// The SSH tunnel uses the same jumpbox configuration as the NATS client.
func NewGardenClientWithImage(stemcellImage string) (*GardenClient, error) {
	gardenAddr := GardenAddress()
	if gardenAddr == "" {
		return nil, fmt.Errorf("GARDEN_ADDRESS environment variable not set")
	}

	logger := lager.NewLogger("garden-test-client")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.INFO))

	// Get SSH tunnel client for dialing through jumpbox
	sshClient, err := utils.GetSSHTunnelClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH tunnel client: %w", err)
	}

	// Create a dialer that uses the SSH tunnel
	dialer := func(network, addr string) (net.Conn, error) {
		// Dial the Garden server through the SSH tunnel
		return sshClient.Dial("tcp", gardenAddr)
	}

	// Create Garden connection with custom dialer
	conn := connection.NewWithDialerAndLogger(dialer, logger)
	gardenClient := client.New(conn)

	// Verify connectivity
	if err := gardenClient.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping Garden server at %s: %w", gardenAddr, err)
	}

	return &GardenClient{
		client:        gardenClient,
		logger:        logger,
		stemcellImage: stemcellImage,
	}, nil
}

// CreateStemcellContainer creates a new privileged container from the stemcell OCI image.
// The container is configured with capabilities needed for nftables and cgroup access.
func (g *GardenClient) CreateStemcellContainer(handle string) error {
	spec := garden.ContainerSpec{
		Handle: handle,
		Image: garden.ImageRef{
			URI: g.stemcellImage,
		},
		Privileged: true,
		Properties: garden.Properties{
			"test": "firewall",
		},
		// Bind mount cgroup filesystem for cgroup detection inside container
		BindMounts: []garden.BindMount{
			{
				SrcPath: "/sys/fs/cgroup",
				DstPath: "/sys/fs/cgroup",
				Mode:    garden.BindMountModeRW,
				Origin:  garden.BindMountOriginHost,
			},
		},
	}

	g.logger.Info("creating-container", lager.Data{"handle": handle, "image": g.stemcellImage})

	container, err := g.client.Create(spec)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	g.container = container
	return nil
}

// RunCommand runs a command in the container and returns stdout, stderr, and exit code.
func (g *GardenClient) RunCommand(path string, args ...string) (stdout, stderr string, exitCode int, err error) {
	if g.container == nil {
		return "", "", -1, fmt.Errorf("no container created")
	}

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

	process, err := g.container.Run(processSpec, processIO)
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to run command: %w", err)
	}

	exitCode, err = process.Wait()
	if err != nil {
		return stdoutBuf.String(), stderrBuf.String(), exitCode, fmt.Errorf("failed waiting for command: %w", err)
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}

// RunCommandWithTimeout runs a command with a timeout.
func (g *GardenClient) RunCommandWithTimeout(timeout time.Duration, path string, args ...string) (stdout, stderr string, exitCode int, err error) {
	done := make(chan struct{})
	var stdoutResult, stderrResult string
	var exitResult int
	var errResult error

	go func() {
		stdoutResult, stderrResult, exitResult, errResult = g.RunCommand(path, args...)
		close(done)
	}()

	select {
	case <-done:
		return stdoutResult, stderrResult, exitResult, errResult
	case <-time.After(timeout):
		return "", "", -1, fmt.Errorf("command timed out after %s", timeout)
	}
}

// StreamIn copies a file into the container at the specified path.
// The file is streamed as a tar archive.
func (g *GardenClient) StreamIn(localPath, containerPath string) error {
	if g.container == nil {
		return fmt.Errorf("no container created")
	}

	// Read the local file
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	// Get file info for permissions
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	// Create tar archive in memory
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Add the file to the tar
	fileName := filepath.Base(localPath)
	header := &tar.Header{
		Name: fileName,
		Mode: int64(info.Mode()),
		Size: int64(len(data)),
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("failed to write tar content: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Stream the tar into the container
	spec := garden.StreamInSpec{
		Path:      containerPath,
		User:      "root",
		TarStream: &buf,
	}

	if err := g.container.StreamIn(spec); err != nil {
		return fmt.Errorf("failed to stream into container: %w", err)
	}

	return nil
}

// StreamInContent streams raw content as a file into the container.
func (g *GardenClient) StreamInContent(content []byte, fileName, containerPath string, mode int64) error {
	if g.container == nil {
		return fmt.Errorf("no container created")
	}

	// Create tar archive in memory
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	header := &tar.Header{
		Name: fileName,
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

	spec := garden.StreamInSpec{
		Path:      containerPath,
		User:      "root",
		TarStream: &buf,
	}

	if err := g.container.StreamIn(spec); err != nil {
		return fmt.Errorf("failed to stream into container: %w", err)
	}

	return nil
}

// StreamOut reads a file from the container.
func (g *GardenClient) StreamOut(containerPath string) ([]byte, error) {
	if g.container == nil {
		return nil, fmt.Errorf("no container created")
	}

	spec := garden.StreamOutSpec{
		Path: containerPath,
		User: "root",
	}

	reader, err := g.container.StreamOut(spec)
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

// GetContainerInfo returns information about the container.
func (g *GardenClient) GetContainerInfo() (garden.ContainerInfo, error) {
	if g.container == nil {
		return garden.ContainerInfo{}, fmt.Errorf("no container created")
	}
	return g.container.Info()
}

// Handle returns the container handle.
func (g *GardenClient) Handle() string {
	if g.container == nil {
		return ""
	}
	return g.container.Handle()
}

// Cleanup destroys the container if it exists.
func (g *GardenClient) Cleanup() error {
	if g.container == nil {
		return nil
	}

	handle := g.container.Handle()
	g.logger.Info("destroying-container", lager.Data{"handle": handle})

	// First try to stop any running processes
	if err := g.container.Stop(true); err != nil {
		g.logger.Error("failed-to-stop-container", err)
	}

	// Then destroy the container
	if err := g.client.Destroy(handle); err != nil {
		return fmt.Errorf("failed to destroy container: %w", err)
	}

	g.container = nil
	return nil
}

// ListContainers lists all containers with optional property filter.
// Returns container handles.
func (g *GardenClient) ListContainers(properties garden.Properties) ([]string, error) {
	containers, err := g.client.Containers(properties)
	if err != nil {
		return nil, err
	}
	handles := make([]string, len(containers))
	for i, c := range containers {
		handles[i] = c.Handle()
	}
	return handles, nil
}

// DestroyContainer destroys a container by handle.
func (g *GardenClient) DestroyContainer(handle string) error {
	return g.client.Destroy(handle)
}

// PrepareAgentEnvironment sets up the container with necessary directories and configs
// for running the bosh-agent.
//
// This includes unmounting bind-mounted files that Garden creates from the host.
// Garden bind-mounts /etc/resolv.conf, /etc/hosts, and /etc/hostname from the host
// into the container. Without unmounting these, the BOSH agent's networking setup
// (particularly resolvconf) fails with "Device or resource busy" when trying to
// modify these files.
//
// This is the same approach used by bosh-warden-cpi to enable Jammy/Noble stemcells
// to work in Garden containers. See:
// https://github.com/cloudfoundry/bosh-warden-cpi-release/blob/main/src/bosh-warden-cpi/vm/warden_creator.go
func (g *GardenClient) PrepareAgentEnvironment() error {
	// First, unmount Garden's bind-mounted files to allow the agent to modify them.
	// Garden bind-mounts /etc/resolv.conf, /etc/hosts, /etc/hostname from the host.
	// Without unmounting, the agent's resolvconf command fails with "Device or resource busy".
	// This is the same approach used by bosh-warden-cpi.
	//
	// We use a single shell command to handle all unmounts, ignoring errors for files
	// that might not be bind-mounted (e.g., in some container configurations).
	unmountScript := `
umount /etc/resolv.conf 2>/dev/null || true
umount /etc/hosts 2>/dev/null || true
umount /etc/hostname 2>/dev/null || true
`
	stdout, stderr, exitCode, err := g.RunCommand("sh", "-c", unmountScript)
	if err != nil {
		return fmt.Errorf("failed to unmount bind-mounted files: %w (stdout: %s, stderr: %s)", err, stdout, stderr)
	}
	if exitCode != 0 {
		// This is unlikely since we use || true, but log it anyway
		g.logger.Info("unmount-warning", lager.Data{
			"exitCode": exitCode,
			"stdout":   stdout,
			"stderr":   stderr,
		})
	}

	// Create necessary directories
	commands := [][]string{
		{"mkdir", "-p", "/var/vcap/bosh/bin"},
		{"mkdir", "-p", "/var/vcap/bosh/log"},
		{"mkdir", "-p", "/var/vcap/data"},
		{"mkdir", "-p", "/var/vcap/data/sys"},
		{"mkdir", "-p", "/var/vcap/monit/job"},
	}

	for _, cmd := range commands {
		stdout, stderr, exitCode, err := g.RunCommand(cmd[0], cmd[1:]...)
		if err != nil {
			return fmt.Errorf("failed to run %v: %w (stdout: %s, stderr: %s)", cmd, err, stdout, stderr)
		}
		if exitCode != 0 {
			return fmt.Errorf("command %v failed with exit code %d (stdout: %s, stderr: %s)", cmd, exitCode, stdout, stderr)
		}
	}

	return nil
}

// GetCgroupVersion detects the cgroup version inside the container.
// Returns "v1", "v2", or "hybrid".
func (g *GardenClient) GetCgroupVersion() (string, error) {
	// Check for cgroup v2 unified hierarchy
	stdout, stderr, exitCode, err := g.RunCommand("sh", "-c", "test -f /sys/fs/cgroup/cgroup.controllers && echo v2")
	if err != nil {
		return "", fmt.Errorf("failed to check cgroup version: %w (stderr: %s)", err, stderr)
	}

	if exitCode == 0 && strings.TrimSpace(stdout) == "v2" {
		return "v2", nil
	}

	// Check for cgroup v1
	stdout, stderr, exitCode, err = g.RunCommand("sh", "-c", "test -d /sys/fs/cgroup/cpu && echo v1")
	if err != nil {
		return "", fmt.Errorf("failed to check cgroup v1: %w (stderr: %s)", err, stderr)
	}

	if exitCode == 0 && strings.TrimSpace(stdout) == "v1" {
		// Check if it's hybrid (both v1 and unified mount exist)
		stdout2, _, exitCode2, _ := g.RunCommand("sh", "-c", "test -d /sys/fs/cgroup/unified && echo hybrid")
		if exitCode2 == 0 && strings.TrimSpace(stdout2) == "hybrid" {
			return "hybrid", nil
		}
		return "v1", nil
	}

	return "unknown", nil
}

// CheckNftablesAvailable checks if nftables is available in the container.
// Deprecated: Use CheckNftablesKernelSupport with nft-dump instead.
func (g *GardenClient) CheckNftablesAvailable() (bool, error) {
	_, _, exitCode, err := g.RunCommand("which", "nft")
	if err != nil {
		return false, err
	}
	return exitCode == 0, nil
}

// NftDumpBinaryPath is the path where the nft-dump binary is installed in containers.
const NftDumpBinaryPath = "/var/vcap/bosh/bin/nft-dump"

// InstallNftDump copies the nft-dump utility into the container.
// If the binary doesn't exist, it will be built automatically.
func (g *GardenClient) InstallNftDump() error {
	// Try to find the binary in common locations
	paths := []string{
		"nft-dump-linux-amd64",
		"../../nft-dump-linux-amd64",
	}

	var foundPath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			foundPath = p
			break
		}
	}

	// If not found, try to build it
	if foundPath == "" {
		g.logger.Info("nft-dump-build", lager.Data{"message": "nft-dump binary not found, building it..."})

		// Determine the source directory - look for integration/nftdump/main.go
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
			return fmt.Errorf("nft-dump source not found in %v and binary not found in %v", sourcePaths, paths)
		}

		// Build the binary
		outputPath := "nft-dump-linux-amd64"
		cmd := exec.Command("go", "build", "-o", outputPath, sourceDir)
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to build nft-dump: %w, output: %s", err, string(output))
		}
		g.logger.Info("nft-dump-build", lager.Data{"message": "Built nft-dump binary", "path": outputPath})
		foundPath = outputPath
	}

	// Stream the binary into the container
	if err := g.StreamIn(foundPath, "/var/vcap/bosh/bin/"); err != nil {
		return fmt.Errorf("failed to copy nft-dump binary: %w", err)
	}

	// Rename and make executable
	stdout, stderr, exitCode, err := g.RunCommand("sh", "-c",
		"mv /var/vcap/bosh/bin/nft-dump-linux-amd64 /var/vcap/bosh/bin/nft-dump && chmod +x /var/vcap/bosh/bin/nft-dump")
	if err != nil {
		return fmt.Errorf("failed to setup nft-dump binary: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("failed to setup nft-dump binary: exit %d, stdout: %s, stderr: %s", exitCode, stdout, stderr)
	}

	return nil
}

// CheckNftablesKernelSupport checks if the kernel supports nftables.
// Uses the nft-dump utility instead of the nft CLI.
func (g *GardenClient) CheckNftablesKernelSupport() (bool, error) {
	_, _, exitCode, err := g.RunCommand(NftDumpBinaryPath, "check")
	if err != nil {
		return false, err
	}
	return exitCode == 0, nil
}

// NftDumpTables returns YAML output listing all nftables tables.
func (g *GardenClient) NftDumpTables() (string, error) {
	stdout, stderr, exitCode, err := g.RunCommand(NftDumpBinaryPath, "tables")
	if err != nil {
		return "", fmt.Errorf("failed to run nft-dump tables: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("nft-dump tables failed: exit %d, stderr: %s", exitCode, stderr)
	}
	return stdout, nil
}

// NftDumpTable returns YAML output for a specific table.
func (g *GardenClient) NftDumpTable(family, name string) (string, error) {
	stdout, stderr, exitCode, err := g.RunCommand(NftDumpBinaryPath, "table", family, name)
	if err != nil {
		return "", fmt.Errorf("failed to run nft-dump table: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("nft-dump table failed: exit %d, stderr: %s", exitCode, stderr)
	}
	return stdout, nil
}

// NftDumpDelete deletes a specific nftables table.
func (g *GardenClient) NftDumpDelete(family, name string) error {
	stdout, stderr, exitCode, err := g.RunCommand(NftDumpBinaryPath, "delete", family, name)
	if err != nil {
		return fmt.Errorf("failed to run nft-dump delete: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("nft-dump delete failed: exit %d, stdout: %s, stderr: %s", exitCode, stdout, stderr)
	}
	return nil
}
