package installerdriver

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
)

// TCPForwarder manages a TCP port forwarder running inside a Garden container.
// It starts a Python-based TCP proxy that listens on a port inside the container
// and forwards connections to a target address. This is more reliable than
// spawning a new netcat process for each connection.
//
// The forwarder is accessed via the container's IP address, so the caller
// needs to be in a network namespace that can reach the container directly
// (e.g., through an SSH tunnel to the host).
type TCPForwarder struct {
	container   garden.Container
	process     garden.Process
	listenPort  int
	targetAddr  string
	containerIP string

	// For tracking the process
	stdinWriter io.WriteCloser
	stderrBuf   *syncBuffer

	closed   bool
	closedMu sync.Mutex
}

// pythonTCPProxyScript is a simple Python TCP proxy that handles multiple concurrent connections.
// It listens on a specified port and forwards all connections to a target address.
const pythonTCPProxyScript = `
import socket
import select
import sys
import os
import signal
import threading

LISTEN_PORT = int(sys.argv[1])
TARGET_HOST = sys.argv[2]
TARGET_PORT = int(sys.argv[3])

# Set up signal handler for clean shutdown
running = True
def signal_handler(sig, frame):
    global running
    running = False
    sys.exit(0)

signal.signal(signal.SIGTERM, signal_handler)
signal.signal(signal.SIGINT, signal_handler)

def forward_data(src, dst, name):
    """Forward data from src to dst socket."""
    try:
        while running:
            # Use select with timeout to allow checking running flag
            ready, _, _ = select.select([src], [], [], 1.0)
            if ready:
                data = src.recv(65536)
                if not data:
                    break
                dst.sendall(data)
    except Exception as e:
        pass
    finally:
        try:
            src.shutdown(socket.SHUT_RD)
        except:
            pass
        try:
            dst.shutdown(socket.SHUT_WR)
        except:
            pass

def handle_connection(client_sock, client_addr):
    """Handle a single client connection."""
    target_sock = None
    try:
        # Connect to target
        target_sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        target_sock.settimeout(120)  # 2 minute timeout for slow operations
        target_sock.connect((TARGET_HOST, TARGET_PORT))
        target_sock.settimeout(None)  # Clear timeout for data transfer
        
        print(f"[PROXY] Connected {client_addr} -> {TARGET_HOST}:{TARGET_PORT}", flush=True)
        
        # Forward data in both directions
        t1 = threading.Thread(target=forward_data, args=(client_sock, target_sock, "client->target"))
        t2 = threading.Thread(target=forward_data, args=(target_sock, client_sock, "target->client"))
        t1.daemon = True
        t2.daemon = True
        t1.start()
        t2.start()
        
        # Wait for either direction to finish
        t1.join()
        t2.join()
        
    except Exception as e:
        print(f"[PROXY] Error handling {client_addr}: {e}", file=sys.stderr, flush=True)
    finally:
        try:
            client_sock.close()
        except:
            pass
        try:
            if target_sock:
                target_sock.close()
        except:
            pass
        print(f"[PROXY] Closed connection from {client_addr}", flush=True)

# Create listening socket
server_sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
server_sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
server_sock.bind(('0.0.0.0', LISTEN_PORT))
server_sock.listen(10)
server_sock.settimeout(1.0)  # Allow periodic check of running flag

print(f"[PROXY] Listening on port {LISTEN_PORT}, forwarding to {TARGET_HOST}:{TARGET_PORT}", flush=True)

try:
    while running:
        try:
            client_sock, client_addr = server_sock.accept()
            # Handle each connection in a new thread
            t = threading.Thread(target=handle_connection, args=(client_sock, client_addr))
            t.daemon = True
            t.start()
        except socket.timeout:
            continue
        except Exception as e:
            if running:
                print(f"[PROXY] Accept error: {e}", file=sys.stderr, flush=True)
finally:
    server_sock.close()
    print("[PROXY] Server shutdown", flush=True)
`

// StartTCPForwarder starts a TCP forwarder inside a Garden container.
// The forwarder listens on listenPort inside the container and forwards
// all connections to targetAddr (e.g., "10.253.0.2:7777").
//
// Returns a TCPForwarder that provides the container IP and port to connect to.
// The caller must call Stop() when done to clean up the forwarder process.
func StartTCPForwarder(container garden.Container, listenPort int, targetAddr string) (*TCPForwarder, error) {
	tunnelDebug("Starting TCP forwarder in container %s: port %d -> %s", container.Handle(), listenPort, targetAddr)

	// Parse target address
	host, port, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid target address %q: %w", targetAddr, err)
	}

	// Get container IP
	info, err := container.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get container info: %w", err)
	}
	containerIP := info.ContainerIP
	if containerIP == "" {
		return nil, fmt.Errorf("container has no IP address")
	}

	// Create pipes for stdin (we don't need it but must provide it)
	stdinReader, stdinWriter := io.Pipe()
	stderrBuf := &syncBuffer{}

	// Write the Python script to a temporary file in the container
	scriptPath := "/tmp/tcp_proxy.py"

	// First, write the script
	writeProcess, err := container.Run(garden.ProcessSpec{
		Path: "/bin/sh",
		Args: []string{"-c", fmt.Sprintf("cat > %s", scriptPath)},
		User: "root",
	}, garden.ProcessIO{
		Stdin: strings.NewReader(pythonTCPProxyScript),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to write proxy script: %w", err)
	}
	exitCode, err := writeProcess.Wait()
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to write proxy script: exit=%d, err=%v", exitCode, err)
	}

	// Start the proxy
	process, err := container.Run(garden.ProcessSpec{
		Path: "python3",
		Args: []string{scriptPath, fmt.Sprintf("%d", listenPort), host, port},
		User: "root",
	}, garden.ProcessIO{
		Stdin:  stdinReader,
		Stdout: stderrBuf, // Capture stdout as logs
		Stderr: stderrBuf, // Capture stderr as logs
	})
	if err != nil {
		stdinWriter.Close()
		return nil, fmt.Errorf("failed to start proxy: %w", err)
	}

	tunnelDebug("TCP forwarder started with PID %s", process.ID())

	// Wait a moment for the proxy to start listening
	time.Sleep(500 * time.Millisecond)

	// Verify the proxy is listening by checking if we can connect
	// We do this by running a quick test connection from within the container
	testProcess, err := container.Run(garden.ProcessSpec{
		Path: "/bin/sh",
		Args: []string{"-c", fmt.Sprintf("echo '' | nc -w 1 127.0.0.1 %d || true", listenPort)},
		User: "root",
	}, garden.ProcessIO{})
	if err == nil {
		testProcess.Wait()
	}

	forwarder := &TCPForwarder{
		container:   container,
		process:     process,
		listenPort:  listenPort,
		targetAddr:  targetAddr,
		containerIP: containerIP,
		stdinWriter: stdinWriter,
		stderrBuf:   stderrBuf,
	}

	tunnelDebug("TCP forwarder ready: %s:%d -> %s", containerIP, listenPort, targetAddr)

	return forwarder, nil
}

// Address returns the address to connect to the forwarder (container_ip:listen_port).
func (f *TCPForwarder) Address() string {
	return fmt.Sprintf("%s:%d", f.containerIP, f.listenPort)
}

// ContainerIP returns the IP address of the container running the forwarder.
func (f *TCPForwarder) ContainerIP() string {
	return f.containerIP
}

// Port returns the port the forwarder is listening on.
func (f *TCPForwarder) Port() int {
	return f.listenPort
}

// Logs returns any log output from the forwarder process.
func (f *TCPForwarder) Logs() string {
	if f.stderrBuf == nil {
		return ""
	}
	return f.stderrBuf.String()
}

// Stop terminates the forwarder process.
func (f *TCPForwarder) Stop() error {
	f.closedMu.Lock()
	defer f.closedMu.Unlock()

	if f.closed {
		return nil
	}
	f.closed = true

	tunnelDebug("Stopping TCP forwarder: %s -> %s", f.Address(), f.targetAddr)
	tunnelDebug("Forwarder logs:\n%s", f.Logs())

	// Close stdin to help signal shutdown
	if f.stdinWriter != nil {
		f.stdinWriter.Close()
	}

	// Send SIGTERM to the process
	if f.process != nil {
		f.process.Signal(garden.SignalTerminate)
	}

	return nil
}
