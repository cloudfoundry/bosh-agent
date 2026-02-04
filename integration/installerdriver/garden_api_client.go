package installerdriver

import (
	"fmt"
	"io"
	"net"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager/v3"
	"golang.org/x/crypto/ssh"
)

// ensureLogger returns a valid logger, creating a no-op logger if nil is passed.
// This prevents panics in the Garden client library which doesn't handle nil loggers.
func ensureLogger(logger lager.Logger, name string) lager.Logger {
	if logger != nil {
		return logger
	}
	// Create a logger that discards all output
	l := lager.NewLogger(name)
	l.RegisterSink(lager.NewWriterSink(io.Discard, lager.ERROR))
	return l
}

// NewGardenAPIClient creates a Garden client that connects through an SSH tunnel.
// The sshClient is used to dial the Garden server at the specified address.
// Address should be in "host:port" format (e.g., "10.0.0.1:7777").
// If logger is nil, a no-op logger will be created to prevent panics.
func NewGardenAPIClient(sshClient *ssh.Client, address string, logger lager.Logger) (garden.Client, error) {
	logger = ensureLogger(logger, "garden-ssh-client")

	dialer := func(network, addr string) (net.Conn, error) {
		return sshClient.Dial("tcp", address)
	}

	conn := connection.NewWithDialerAndLogger(dialer, logger)
	gardenClient := client.New(conn)

	// Verify connectivity
	if err := gardenClient.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping Garden at %s: %w", address, err)
	}

	return gardenClient, nil
}

// NewGardenAPIClientDirect creates a Garden client that connects directly to the address.
// This is useful for local testing without SSH tunnels.
// Address should be in "host:port" format (e.g., "127.0.0.1:7777").
// If logger is nil, a no-op logger will be created to prevent panics.
func NewGardenAPIClientDirect(address string, logger lager.Logger) (garden.Client, error) {
	logger = ensureLogger(logger, "garden-direct-client")

	dialer := func(network, addr string) (net.Conn, error) {
		return net.Dial("tcp", address)
	}

	conn := connection.NewWithDialerAndLogger(dialer, logger)
	gardenClient := client.New(conn)

	// Verify connectivity
	if err := gardenClient.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping Garden at %s: %w", address, err)
	}

	return gardenClient, nil
}

// NewGardenAPIClientThroughContainer creates a Garden client that connects through
// a Garden container using netcat. This is useful for reaching nested containers
// that are only accessible from within an intermediate container's network namespace.
//
// For example, to connect to L2 Garden (running inside L1 container), the traffic
// must be tunneled through L1 because L2's IP is only reachable from L1's namespace.
//
// The container parameter is the Garden container to tunnel through (e.g., L1 container).
// The address parameter is the target Garden server address (e.g., "10.253.0.2:7777").
// If logger is nil, a no-op logger will be created to prevent panics.
//
// Note: This creates a new netcat process for each connection, which is less efficient
// than a persistent TCP forwarder but avoids the need to install external tools like socat.
func NewGardenAPIClientThroughContainer(container garden.Container, address string, logger lager.Logger) (garden.Client, error) {
	logger = ensureLogger(logger, "garden-tunnel-client")

	dialer := func(network, addr string) (net.Conn, error) {
		return DialThroughContainer(container, address)
	}

	conn := connection.NewWithDialerAndLogger(dialer, logger)
	gardenClient := client.New(conn)

	// Verify connectivity
	if err := gardenClient.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping Garden at %s through container: %w", address, err)
	}

	return gardenClient, nil
}

// NewGardenAPIClientWithForwarder creates a Garden client that connects through
// a TCP forwarder running inside a Garden container. This is more reliable than
// NewGardenAPIClientThroughContainer for long-running operations because it uses
// a persistent TCP proxy instead of spawning a new netcat process for each request.
//
// The sshClient parameter is an SSH client connected to a host that can reach
// the container's network namespace (e.g., the host running the container).
// The forwarder parameter is a running TCPForwarder started with StartTCPForwarder.
// If logger is nil, a no-op logger will be created to prevent panics.
//
// Returns the Garden client. The caller is responsible for stopping the forwarder
// when done using the client.
func NewGardenAPIClientWithForwarder(sshClient *ssh.Client, forwarder *TCPForwarder, logger lager.Logger) (garden.Client, error) {
	logger = ensureLogger(logger, "garden-forwarder-client")

	forwarderAddr := forwarder.Address()
	tunnelDebug("Creating Garden client through forwarder at %s", forwarderAddr)

	dialer := func(network, addr string) (net.Conn, error) {
		tunnelDebug("Dialing forwarder at %s via SSH", forwarderAddr)
		return sshClient.Dial("tcp", forwarderAddr)
	}

	conn := connection.NewWithDialerAndLogger(dialer, logger)
	gardenClient := client.New(conn)

	// Verify connectivity
	if err := gardenClient.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping Garden through forwarder at %s: %w", forwarderAddr, err)
	}

	tunnelDebug("Garden client connected through forwarder")
	return gardenClient, nil
}
