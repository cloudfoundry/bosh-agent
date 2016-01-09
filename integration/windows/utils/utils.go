package utils

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

const (
	agentID = "123-456-789"
)

type Agent struct {
	ID   string
	tail *gexec.Session
}

func StartAgent() (Agent, error) {
	command := exec.Command("./setup.sh")
	command.Env = append(os.Environ(), fmt.Sprintf("AGENT_ID=%s", agentID))
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		return Agent{}, err
	}
	gomega.Eventually(session, 300*time.Second).Should(gexec.Exit(0))

	agentTail, err := gexec.Start(exec.Command("bash", "-c", "tail -f service_wrapper.*.log"), ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		return Agent{}, err
	}

	gomega.Eventually(agentTail, 30*time.Second).Should(gbytes.Say("Subscribing to agent"))
	return Agent{
		ID:   agentID,
		tail: agentTail,
	}, nil
}

func (a Agent) Stop() {
	a.tail.Kill()
}
