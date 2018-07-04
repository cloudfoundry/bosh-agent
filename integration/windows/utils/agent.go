package utils

import (
	"os"
	"os/exec"
	"time"

	"bytes"

	"regexp"

	"fmt"

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
