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

// Compile-time check that we're using garden.Client
var _ garden.Client = nil

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
