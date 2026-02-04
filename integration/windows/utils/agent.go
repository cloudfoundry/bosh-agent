package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func BuildAgent() error {
	command := exec.Command("./build_agent.bash")
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		return err
	}
	gomega.Eventually(session, 40*time.Minute).Should(gexec.Exit(0))
	return nil
}

func AgentIP() string {
	return os.Getenv("AGENT_IP")
}

func AgentNetmask() string {
	return os.Getenv("AGENT_NETMASK")
}

func AgentGateway() string {
	return os.Getenv("AGENT_GATEWAY")
}

func FakeDirectorIP() string {
	return os.Getenv("FAKE_DIRECTOR_IP")
}

func BlobstoreURI() string {
	return fmt.Sprintf("http://%s:25250", FakeDirectorIP())
}

func AgentDir() string {
	windowsIntegrationPath, _ := os.Getwd() //nolint:errcheck
	integrationPath := filepath.Dir(windowsIntegrationPath)
	agentDir := filepath.Dir(integrationPath)
	return agentDir
}

func JumpboxIP() string {
	return os.Getenv("JUMPBOX_IP")
}

func JumpboxUsername() string {
	return os.Getenv("JUMPBOX_USERNAME")
}

func JumpboxKeyPath() string {
	return os.Getenv("JUMPBOX_KEY_PATH")
}

func getPemContents(path string) string {
	contents, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(contents)
}

func NatsCA() string {
	return getPemContents(os.Getenv("NATS_CA_PATH"))
}

func NatsCertificate() string {
	return getPemContents(os.Getenv("NATS_CERTIFICATE_PATH"))
}

func NatsPrivateKey() string {
	return getPemContents(os.Getenv("NATS_PRIVATE_KEY_PATH"))
}

func GetSSHTunnelClient() (*ssh.Client, error) {
	key, err := os.ReadFile(JumpboxKeyPath())
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", JumpboxIP()), &ssh.ClientConfig{
		User:            JumpboxUsername(),
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second, // Connection timeout
	})
	if err != nil {
		return nil, err
	}

	// Start a goroutine to send keepalive requests to prevent the connection from timing out.
	// This is especially important for long-running operations like installing Garden in nested containers.
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			// SendRequest with wantReply=true acts as a keepalive
			_, _, err := sshClient.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				// Connection is dead, stop the goroutine
				return
			}
		}
	}()

	return sshClient, nil
}
