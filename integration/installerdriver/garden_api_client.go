package installerdriver

import (
	"fmt"
	"net"
	"strings"

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

// StartTCPForwarder starts a TCP port forwarder inside the container using socat.
// It listens on listenPort and forwards connections to targetAddr (e.g., "10.253.0.10:7777").
// This is used to reach nested containers (e.g., L2) from outside by tunneling through
// an intermediate container (e.g., L1).
// Returns the address to connect to (containerIP:listenPort).
func StartTCPForwarder(driver *GardenDriver, listenPort uint32, targetAddr string) (string, error) {
	if !driver.IsBootstrapped() {
		return "", fmt.Errorf("driver not bootstrapped")
	}

	// Get container IP to return the full address
	containerIP, err := driver.ContainerIP()
	if err != nil {
		return "", fmt.Errorf("failed to get container IP: %w", err)
	}

	// Step 1: Check if socat is available, install if needed
	checkScript := `command -v socat >/dev/null 2>&1 && echo "socat available" || echo "socat missing"`
	stdout, _, _, err := driver.RunScript(checkScript)
	if err != nil {
		return "", fmt.Errorf("failed to check socat: %w", err)
	}

	if strings.Contains(stdout, "socat missing") {
		// Install socat - run apt-get in a separate command to isolate any trigger issues
		installScript := `
set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -qq -y socat
echo "socat installed"
`
		stdout, stderr, exitCode, err := driver.RunScript(installScript)
		if err != nil {
			return "", fmt.Errorf("failed to install socat: %w (stdout=%s, stderr=%s)", err, stdout, stderr)
		}
		if exitCode != 0 {
			return "", fmt.Errorf("socat installation failed with exit code %d (stdout=%s, stderr=%s)", exitCode, stdout, stderr)
		}
	}

	// Step 2: Kill any existing forwarder on this port
	killScript := fmt.Sprintf(`pkill -f "socat.*TCP-LISTEN:%d" 2>/dev/null || true`, listenPort)
	_, _, _, _ = driver.RunScript(killScript) // Ignore errors

	// Step 3: Start socat forwarder using setsid to fully detach from the process group
	// Using setsid ensures the process survives after the shell exits
	startScript := fmt.Sprintf(`
setsid socat TCP-LISTEN:%d,fork,reuseaddr TCP:%s </dev/null >/tmp/socat-forwarder-%d.log 2>&1 &
disown
sleep 1
echo "forwarder started"
`, listenPort, targetAddr, listenPort)

	stdout, stderr, exitCode, err := driver.RunScript(startScript)
	if err != nil {
		return "", fmt.Errorf("failed to start forwarder: %w (stdout=%s, stderr=%s)", err, stdout, stderr)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("forwarder start script failed with exit code %d (stdout=%s, stderr=%s)", exitCode, stdout, stderr)
	}

	// Step 4: Verify the forwarder is running
	verifyScript := fmt.Sprintf(`
if pgrep -f "socat.*TCP-LISTEN:%d" >/dev/null; then
    echo "Forwarder running on port %d -> %s"
    exit 0
else
    echo "ERROR: Forwarder not running" >&2
    cat /tmp/socat-forwarder-%d.log 2>/dev/null >&2 || true
    exit 1
fi
`, listenPort, listenPort, targetAddr, listenPort)

	stdout, stderr, exitCode, err = driver.RunScript(verifyScript)
	if err != nil {
		return "", fmt.Errorf("failed to verify forwarder: %w (stdout=%s, stderr=%s)", err, stdout, stderr)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("forwarder verification failed with exit code %d (stdout=%s, stderr=%s)", exitCode, stdout, stderr)
	}

	return fmt.Sprintf("%s:%d", containerIP, listenPort), nil
}
