// Package installerdriver provides the Driver interface for executing commands
// and transferring files to target environments (VMs via SSH, Garden containers, etc.).
//
// The Driver abstraction allows the same installation logic to work on bare VMs,
// containers at any nesting level, and other target environments.
package installerdriver

import (
	"errors"
	"io"
)

// BaseDir is the standard BOSH installation directory.
const BaseDir = "/var/vcap"

// ErrNotBootstrapped is returned when a driver method is called before Bootstrap().
var ErrNotBootstrapped = errors.New("driver not bootstrapped: call Bootstrap() first")

// Driver is the interface for executing commands and transferring files
// to a target environment (VM via SSH, Garden container, etc.).
type Driver interface {
	// === Lifecycle ===

	// Bootstrap prepares the target environment.
	// For SSHDriver: creates base directories on the VM.
	// For GardenDriver: creates container with bind mounts, port forwarding.
	// Must be called before any other methods.
	Bootstrap() error

	// Cleanup cleans up resources created by Bootstrap.
	// For SSHDriver: no-op (leave directories for debugging).
	// For GardenDriver: destroys container, removes host-side bind mount directory.
	Cleanup() error

	// IsBootstrapped returns true if Bootstrap() has been called successfully.
	IsBootstrapped() bool

	// === Execution ===

	// RunCommand executes a command and returns stdout, stderr, and exit code.
	RunCommand(path string, args ...string) (stdout, stderr string, exitCode int, err error)

	// RunScript executes a shell script (passed as string content).
	RunScript(script string) (stdout, stderr string, exitCode int, err error)

	// === File Operations ===

	// WriteFile writes content to a file at the given path with the specified mode.
	WriteFile(path string, content []byte, mode int64) error

	// ReadFile reads the content of a file at the given path.
	ReadFile(path string) ([]byte, error)

	// MkdirAll creates a directory and all parent directories.
	MkdirAll(path string, mode int64) error

	// StreamTarball streams a tarball from a reader and extracts it to destDir.
	StreamTarball(r io.Reader, destDir string) error

	// Chmod changes the file mode of the specified path.
	Chmod(path string, mode int64) error

	// === Metadata ===

	// Description returns a human-readable description of the target (for logging).
	Description() string
}
