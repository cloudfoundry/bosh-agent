package gardeninstaller

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/ssh"
)

// SSHDriver implements Driver for VMs accessible via SSH.
type SSHDriver struct {
	client  *ssh.Client
	host    string
	useSudo bool
}

// NewSSHDriver creates a new driver for the given SSH client.
func NewSSHDriver(client *ssh.Client, host string) *SSHDriver {
	return &SSHDriver{client: client, host: host, useSudo: false}
}

// NewSSHDriverWithSudo creates a new driver that runs commands with sudo.
func NewSSHDriverWithSudo(client *ssh.Client, host string) *SSHDriver {
	return &SSHDriver{client: client, host: host, useSudo: true}
}

// Description returns a human-readable description of the target.
func (d *SSHDriver) Description() string {
	return fmt.Sprintf("ssh:%s", d.host)
}

// RunCommand executes a command on the remote host.
func (d *SSHDriver) RunCommand(path string, args ...string) (stdout, stderr string, exitCode int, err error) {
	session, err := d.client.NewSession()
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	// Build command string with proper quoting
	cmd := path
	for _, arg := range args {
		cmd += " " + shellQuote(arg)
	}

	// Wrap with sudo if needed
	if d.useSudo {
		cmd = "sudo " + cmd
	}

	err = session.Run(cmd)
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
			err = nil // Not an error, just non-zero exit
		} else {
			return stdoutBuf.String(), stderrBuf.String(), -1, err
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}

// RunScript executes a shell script on the remote host.
func (d *SSHDriver) RunScript(script string) (stdout, stderr string, exitCode int, err error) {
	session, err := d.client.NewSession()
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf
	session.Stdin = strings.NewReader(script)

	// Use bash for bash-specific syntax like 'source'
	cmd := "bash -s"
	if d.useSudo {
		cmd = "sudo bash -s"
	}

	err = session.Run(cmd)
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
			err = nil
		} else {
			return stdoutBuf.String(), stderrBuf.String(), -1, err
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}

// WriteFile writes content to a file on the remote host.
func (d *SSHDriver) WriteFile(path string, content []byte, mode int64) error {
	session, err := d.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Use cat to write the file - this is simpler than SCP protocol
	session.Stdin = bytes.NewReader(content)
	cmd := fmt.Sprintf("cat > %s && chmod %o %s", shellQuote(path), mode, shellQuote(path))
	if d.useSudo {
		// Use tee for sudo to handle the redirection
		cmd = fmt.Sprintf("sudo tee %s > /dev/null && sudo chmod %o %s", shellQuote(path), mode, shellQuote(path))
	}
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ReadFile reads a file from the remote host.
func (d *SSHDriver) ReadFile(path string) ([]byte, error) {
	session, err := d.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	session.Stdout = &stdout

	cmd := fmt.Sprintf("cat %s", shellQuote(path))
	if d.useSudo {
		cmd = "sudo " + cmd
	}
	if err := session.Run(cmd); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return stdout.Bytes(), nil
}

// MkdirAll creates a directory and all parent directories.
func (d *SSHDriver) MkdirAll(path string, mode int64) error {
	stdout, stderr, exitCode, err := d.RunCommand("mkdir", "-p", path)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("mkdir failed with exit code %d: stdout=%s, stderr=%s", exitCode, stdout, stderr)
	}
	return nil
}

// StreamTarball streams a gzipped tarball and extracts it to destDir on the remote host.
func (d *SSHDriver) StreamTarball(r io.Reader, destDir string) error {
	session, err := d.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Decompress gzip and send to tar on remote
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	session.Stdin = gr
	cmd := fmt.Sprintf("tar -xf - -C %s", shellQuote(destDir))
	if d.useSudo {
		cmd = "sudo " + cmd
	}

	var stderr bytes.Buffer
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("tar extraction failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// StreamTarballFromData is a helper that streams a gzipped tarball from byte data.
func (d *SSHDriver) StreamTarballFromData(data []byte, destDir string) error {
	return d.StreamTarball(bytes.NewReader(data), destDir)
}

// ExtractTarGzToDir extracts a gzipped tar archive to a directory on the remote host.
// This reads the tarball locally and re-creates it for streaming.
func (d *SSHDriver) ExtractTarGzToDir(data []byte, destDir string) error {
	// First ensure the destination directory exists
	if err := d.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Decompress and extract the tarball
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	// Create a new tar with appropriate structure for streaming
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		// Write content if it's a regular file
		if header.Typeflag == tar.TypeReg {
			if _, err := io.Copy(tw, tr); err != nil {
				return fmt.Errorf("failed to copy tar content: %w", err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Stream the uncompressed tar to remote
	session, err := d.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	session.Stdin = &tarBuf
	cmd := fmt.Sprintf("tar -xf - -C %s", shellQuote(destDir))
	if d.useSudo {
		cmd = "sudo " + cmd
	}

	var stderr bytes.Buffer
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("tar extraction failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// Chmod changes the file mode of the specified path.
func (d *SSHDriver) Chmod(path string, mode int64) error {
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

// shellQuote quotes a string for safe use in a shell command.
func shellQuote(s string) string {
	// Use single quotes and escape any single quotes in the string
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// Verify SSHDriver implements Driver
var _ Driver = (*SSHDriver)(nil)
