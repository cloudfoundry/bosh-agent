package installerdriver

import (
	"fmt"
	"net"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager/v3"
	"golang.org/x/crypto/ssh"
)

// NewGardenAPIClient creates a Garden client that connects through an SSH tunnel.
// The sshClient is used to dial the Garden server at the specified address.
// Address should be in "host:port" format (e.g., "10.0.0.1:7777").
func NewGardenAPIClient(sshClient *ssh.Client, address string, logger lager.Logger) (garden.Client, error) {
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
func NewGardenAPIClientDirect(address string, logger lager.Logger) (garden.Client, error) {
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
