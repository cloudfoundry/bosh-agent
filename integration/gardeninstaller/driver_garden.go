package gardeninstaller

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"code.cloudfoundry.org/garden"
)

// GardenDriver implements Driver for Garden containers.
// It uses the Garden API to execute commands and stream files.
type GardenDriver struct {
	container garden.Container
}

// NewGardenDriver creates a new driver for the given Garden container.
func NewGardenDriver(container garden.Container) *GardenDriver {
	return &GardenDriver{container: container}
}

// Description returns a human-readable description of the target.
func (d *GardenDriver) Description() string {
	return fmt.Sprintf("garden-container:%s", d.container.Handle())
}

// RunCommand executes a command in the container.
func (d *GardenDriver) RunCommand(path string, args ...string) (stdout, stderr string, exitCode int, err error) {
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
	return d.RunCommand("sh", "-c", script)
}

// WriteFile writes content to a file in the container.
func (d *GardenDriver) WriteFile(path string, content []byte, mode int64) error {
	// Create tar archive with the file
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Get the filename from the path
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
	stdout, stderr, exitCode, err := d.RunCommand("mkdir", "-p", path)
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
	// Garden's StreamIn expects an uncompressed tar, so we need to decompress first
	gr, err := gzip.NewReader(r)
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
	modeStr := fmt.Sprintf("%o", mode)
	stdout, stderr, exitCode, err := d.RunCommand("chmod", modeStr, path)
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
