package gardeninstaller

import (
	"io"
)

// Driver is the interface for executing commands and transferring files
// to a target environment (VM via SSH, Garden container, etc.).
type Driver interface {
	// RunCommand executes a command and returns stdout, stderr, and exit code.
	RunCommand(path string, args ...string) (stdout, stderr string, exitCode int, err error)

	// RunScript executes a shell script (passed as string content).
	RunScript(script string) (stdout, stderr string, exitCode int, err error)

	// WriteFile writes content to a file at the given path with the specified mode.
	WriteFile(path string, content []byte, mode int64) error

	// ReadFile reads the content of a file at the given path.
	ReadFile(path string) ([]byte, error)

	// MkdirAll creates a directory and all parent directories.
	MkdirAll(path string, mode int64) error

	// StreamTarball streams a tarball from a local reader and extracts it to destDir.
	// This is used to transfer compiled packages efficiently.
	StreamTarball(r io.Reader, destDir string) error

	// Chmod changes the file mode of the specified path.
	Chmod(path string, mode int64) error

	// Description returns a human-readable description of the target (for logging).
	Description() string
}
