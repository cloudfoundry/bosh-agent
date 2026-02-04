package installerdriver

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
)

// debugLog is a package-level flag to enable debug logging for tunnel connections
var debugLog = os.Getenv("DEBUG_TUNNEL") != ""

func tunnelDebug(format string, args ...interface{}) {
	if debugLog {
		log.Printf("[TUNNEL] "+format, args...)
	}
}

// GardenProcessConn implements net.Conn by wrapping a Garden process's stdin/stdout.
// This allows TCP-like communication through a Garden container using netcat (nc).
//
// The connection works by running `nc <host> <port>` inside the container and
// piping data through the process's stdin (for writes) and stdout (for reads).
type GardenProcessConn struct {
	container garden.Container
	process   garden.Process
	target    string

	// Pipes for communication
	stdinWriter  io.WriteCloser
	stdoutReader io.Reader

	// Process exit tracking
	exitCh    chan int
	exitErr   chan error
	stderrBuf *syncBuffer

	// Connection state
	closed   bool
	closedMu sync.Mutex

	// Timeouts (not fully implemented - Garden processes don't support deadlines)
	readDeadline  time.Time
	writeDeadline time.Time
}

// DialThroughContainer creates a net.Conn that tunnels through a Garden container.
// It runs netcat (nc) inside the container to connect to the target address.
//
// This is useful for reaching nested containers - for example, to connect to L2 Garden
// (which is only reachable from L1's network namespace) from the test runner.
//
// The caller is responsible for closing the connection when done.
func DialThroughContainer(container garden.Container, targetAddr string) (net.Conn, error) {
	return DialThroughContainerWithTimeout(container, targetAddr, 10*time.Second)
}

// DialThroughContainerWithTimeout creates a net.Conn that tunnels through a Garden container
// with a specified timeout for the connection establishment.
func DialThroughContainerWithTimeout(container garden.Container, targetAddr string, timeout time.Duration) (net.Conn, error) {
	tunnelDebug("Dialing %s through container %s", targetAddr, container.Handle())

	// Parse target address
	host, port, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid target address %q: %w", targetAddr, err)
	}

	// Create pipes for stdin/stdout
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	// Capture stderr to detect connection errors
	stderrBuf := &syncBuffer{}

	// Track if the process has exited
	exitCh := make(chan int, 1)
	exitErr := make(chan error, 1)

	// Run netcat to connect to target
	// Using -v for verbose output (connection info on stderr)
	// Using -w 120 for a long connection timeout (container creation can take minutes)
	// Note: We don't use -q0 because it can cause the connection to close before
	// the HTTP response is received (especially for slow operations like container creation)
	tunnelDebug("Starting netcat process: nc -v -w 120 %s %s", host, port)
	process, err := container.Run(garden.ProcessSpec{
		Path: "nc",
		Args: []string{"-v", "-w", "120", host, port},
		User: "root",
	}, garden.ProcessIO{
		Stdin:  stdinReader,
		Stdout: stdoutWriter,
		Stderr: stderrBuf,
	})
	if err != nil {
		stdinWriter.Close()
		stdinReader.Close()
		stdoutReader.Close()
		stdoutWriter.Close()
		tunnelDebug("Failed to run netcat: %v", err)
		return nil, fmt.Errorf("failed to run netcat: %w", err)
	}
	tunnelDebug("Netcat process started with ID %s", process.ID())

	// Start a goroutine to wait for the process to exit
	go func() {
		code, err := process.Wait()
		tunnelDebug("Netcat process %s exited: code=%d, err=%v, stderr=%s", process.ID(), code, err, stderrBuf.String())
		if err != nil {
			exitErr <- err
		} else {
			exitCh <- code
		}
		stdoutWriter.Close()
	}()

	// Wait briefly to see if netcat connects or fails immediately
	// The -v flag makes nc print connection info to stderr
	select {
	case code := <-exitCh:
		// Process exited - this is an error (connection failed)
		stdinWriter.Close()
		stdinReader.Close()
		stdoutReader.Close()
		stderr := stderrBuf.String()
		tunnelDebug("Netcat exited immediately with code %d: %s", code, stderr)
		return nil, fmt.Errorf("netcat exited immediately with code %d connecting to %s: %s", code, targetAddr, stderr)
	case err := <-exitErr:
		stdinWriter.Close()
		stdinReader.Close()
		stdoutReader.Close()
		stderr := stderrBuf.String()
		tunnelDebug("Netcat process error: %v, stderr: %s", err, stderr)
		return nil, fmt.Errorf("netcat process error connecting to %s: %w, stderr: %s", targetAddr, err, stderr)
	case <-time.After(100 * time.Millisecond):
		// Process is still running after 100ms, assume connection is being established
		// We can't know for sure if it connected, but nc -v will print to stderr on success
		tunnelDebug("Netcat still running after 100ms, assuming connection in progress")
	}

	conn := &GardenProcessConn{
		container:    container,
		process:      process,
		target:       targetAddr,
		stdinWriter:  stdinWriter,
		stdoutReader: stdoutReader,
		exitCh:       exitCh,
		exitErr:      exitErr,
		stderrBuf:    stderrBuf,
	}

	tunnelDebug("Connection established to %s", targetAddr)

	return conn, nil
}

// syncBuffer is a thread-safe buffer for capturing stderr
type syncBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// Read reads data from the connection (from netcat's stdout).
func (c *GardenProcessConn) Read(b []byte) (int, error) {
	c.closedMu.Lock()
	if c.closed {
		c.closedMu.Unlock()
		return 0, io.EOF
	}
	c.closedMu.Unlock()

	return c.stdoutReader.Read(b)
}

// Write writes data to the connection (to netcat's stdin).
func (c *GardenProcessConn) Write(b []byte) (int, error) {
	c.closedMu.Lock()
	if c.closed {
		c.closedMu.Unlock()
		return 0, io.ErrClosedPipe
	}
	c.closedMu.Unlock()

	return c.stdinWriter.Write(b)
}

// Close closes the connection by closing stdin (which causes netcat to exit).
func (c *GardenProcessConn) Close() error {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	// Close stdin to signal netcat to exit
	if c.stdinWriter != nil {
		c.stdinWriter.Close()
	}

	// Signal the process to terminate if it's still running
	if c.process != nil {
		c.process.Signal(garden.SignalTerminate)
	}

	return nil
}

// LocalAddr returns a dummy local address (Garden processes don't have real addresses).
func (c *GardenProcessConn) LocalAddr() net.Addr {
	return &gardenProcessAddr{network: "garden", addr: "container"}
}

// RemoteAddr returns the target address.
func (c *GardenProcessConn) RemoteAddr() net.Addr {
	return &gardenProcessAddr{network: "tcp", addr: c.target}
}

// SetDeadline sets both read and write deadlines.
// Note: Garden processes don't support true deadlines, so this is best-effort.
func (c *GardenProcessConn) SetDeadline(t time.Time) error {
	c.readDeadline = t
	c.writeDeadline = t
	return nil
}

// SetReadDeadline sets the read deadline.
// Note: Garden processes don't support true deadlines, so this is best-effort.
func (c *GardenProcessConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

// SetWriteDeadline sets the write deadline.
// Note: Garden processes don't support true deadlines, so this is best-effort.
func (c *GardenProcessConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}

// gardenProcessAddr implements net.Addr for Garden process connections.
type gardenProcessAddr struct {
	network string
	addr    string
}

func (a *gardenProcessAddr) Network() string { return a.network }
func (a *gardenProcessAddr) String() string  { return a.addr }

// Verify GardenProcessConn implements net.Conn
var _ net.Conn = (*GardenProcessConn)(nil)
