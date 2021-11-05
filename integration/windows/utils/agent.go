package utils

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"bytes"

	"regexp"

	"github.com/onsi/ginkgo"
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

func StartVagrant(vmName, provider string, osVersion string) error {
	if len(provider) == 0 {
		provider = "virtualbox"
	}
	command := exec.Command("vagrant", "up", vmName, fmt.Sprintf("--provider=%s", provider), "--provision")
	command.Env = append(os.Environ(), "WINDOWS_OS_VERSION="+osVersion)
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		return err
	}
	gomega.Eventually(session, 40*time.Minute).Should(gexec.Exit(0))

	return nil
}

func RetrievePrivateIP(vmName string) (string, error) {
	command := exec.Command("vagrant", "ssh", vmName, "-c", `hostname -I`)
	stdout := new(bytes.Buffer)
	session, err := gexec.Start(command, stdout, ginkgo.GinkgoWriter)
	if err != nil {
		return "", err
	}
	gomega.Eventually(session, 20*time.Second).Should(gexec.Exit(0))

	privateIPMatcher, err := regexp.Compile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	return privateIPMatcher.FindString(stdout.String()), nil
}

func RetrievePublicIP(vmName string) (string, error) {
	command := exec.Command("vagrant", "ssh-config", vmName)
	stdout := new(bytes.Buffer)
	session, err := gexec.Start(command, stdout, ginkgo.GinkgoWriter)
	if err != nil {
		return "", err
	}
	gomega.Eventually(session, 20*time.Second).Should(gexec.Exit(0))

	hostnameMatcher, err := regexp.Compile(`HostName\s([a-zA-Z0-9\.-]*)\n`)
	return hostnameMatcher.FindStringSubmatch(stdout.String())[1], nil
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
	windowsIntegrationPath, _ := os.Getwd()
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
	contents, err := ioutil.ReadFile(path)
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
	key, err := ioutil.ReadFile(JumpboxKeyPath())
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
	})
	return sshClient, nil
}
