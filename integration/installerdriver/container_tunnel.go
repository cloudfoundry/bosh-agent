package installerdriver

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
)

// ContainerTunnel provides a persistent TCP tunnel through a Garden container.
// It starts a single netcat process in the container and creates a local TCP
// listener that forwards traffic through it. This is more efficient than
// spawning a new netcat process for each connection.
//
// The tunnel supports HTTP/1.1 keepalive connections because it maintains
// a persistent connection to the target.
type ContainerTunnel struct {
	container    garden.Container
	targetAddr   string
	localAddr    string
	listener     net.Listener
	process      garden.Process
	stdinWriter  io.WriteCloser
	stdinReader  io.Reader
	stdoutReader io.Reader
	stdoutWriter io.WriteCloser
	stderrBuf    *syncBuffer

	// For managing the single active connection
	connMu     sync.Mutex
	activeConn net.Conn

	closed   bool
	closedMu sync.Mutex
	wg       sync.WaitGroup
}

// NewContainerTunnel creates a new tunnel through the container to the target address.
// It starts a netcat process in the container and returns a local address that
// can be used to connect to the target.
//
// The caller must call Close() when done to clean up resources.
func NewContainerTunnel(container garden.Container, targetAddr string) (*ContainerTunnel, error) {
	host, port, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid target address %q: %w", targetAddr, err)
	}

	// Create pipes for stdin/stdout
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	stderrBuf := &syncBuffer{}

	// Start netcat in the container
	// Using -v for verbose output
	// Using -w 0 for no timeout (keep connection open indefinitely)
	// Using -k to keep listening (though we only use one connection)
	tunnelDebug("Starting persistent netcat: nc -v %s %s", host, port)
	process, err := container.Run(garden.ProcessSpec{
		Path: "nc",
		Args: []string{"-v", host, port},
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
		return nil, fmt.Errorf("failed to start netcat: %w", err)
	}

	// Wait briefly for netcat to connect
	time.Sleep(200 * time.Millisecond)

	// Check if netcat exited immediately (connection failed)
	// We can't easily check this without Wait(), so we'll detect it on first use

	// Create a local TCP listener on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		stdinWriter.Close()
		process.Signal(garden.SignalTerminate)
		return nil, fmt.Errorf("failed to create local listener: %w", err)
	}

	tunnel := &ContainerTunnel{
		container:    container,
		targetAddr:   targetAddr,
		localAddr:    listener.Addr().String(),
		listener:     listener,
		process:      process,
		stdinWriter:  stdinWriter,
		stdinReader:  stdinReader,
		stdoutReader: stdoutReader,
		stdoutWriter: stdoutWriter,
		stderrBuf:    stderrBuf,
	}

	// Start accepting connections
	tunnel.wg.Add(1)
	go tunnel.acceptLoop()

	tunnelDebug("Container tunnel ready: local=%s -> container=%s -> target=%s",
		tunnel.localAddr, container.Handle(), targetAddr)

	return tunnel, nil
}

// LocalAddr returns the local address that can be used to connect to the tunnel.
func (t *ContainerTunnel) LocalAddr() string {
	return t.localAddr
}

// Close shuts down the tunnel and cleans up resources.
func (t *ContainerTunnel) Close() error {
	t.closedMu.Lock()
	if t.closed {
		t.closedMu.Unlock()
		return nil
	}
	t.closed = true
	t.closedMu.Unlock()

	tunnelDebug("Closing container tunnel to %s", t.targetAddr)

	// Close listener to stop accepting new connections
	if t.listener != nil {
		t.listener.Close()
	}

	// Close stdin to signal netcat to exit
	if t.stdinWriter != nil {
		t.stdinWriter.Close()
	}

	// Terminate the netcat process
	if t.process != nil {
		t.process.Signal(garden.SignalTerminate)
	}

	// Close any active connection
	t.connMu.Lock()
	if t.activeConn != nil {
		t.activeConn.Close()
	}
	t.connMu.Unlock()

	// Wait for goroutines to finish
	t.wg.Wait()

	return nil
}

func (t *ContainerTunnel) acceptLoop() {
	defer t.wg.Done()

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			t.closedMu.Lock()
			closed := t.closed
			t.closedMu.Unlock()
			if closed {
				return
			}
			tunnelDebug("Accept error: %v", err)
			continue
		}

		// Handle the connection
		// Note: We only support one connection at a time since netcat
		// is a single bidirectional stream
		t.handleConnection(conn)
	}
}

func (t *ContainerTunnel) handleConnection(conn net.Conn) {
	t.connMu.Lock()
	// Close any existing connection
	if t.activeConn != nil {
		t.activeConn.Close()
	}
	t.activeConn = conn
	t.connMu.Unlock()

	tunnelDebug("New connection from %s", conn.RemoteAddr())

	// Forward data bidirectionally
	var wg sync.WaitGroup
	wg.Add(2)

	// conn -> netcat stdin
	go func() {
		defer wg.Done()
		_, err := io.Copy(t.stdinWriter, conn)
		if err != nil {
			tunnelDebug("Error copying to netcat stdin: %v", err)
		}
	}()

	// netcat stdout -> conn
	go func() {
		defer wg.Done()
		_, err := io.Copy(conn, t.stdoutReader)
		if err != nil {
			tunnelDebug("Error copying from netcat stdout: %v", err)
		}
		conn.Close()
	}()

	wg.Wait()
	tunnelDebug("Connection from %s closed", conn.RemoteAddr())
}
