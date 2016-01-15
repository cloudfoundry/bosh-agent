package utils

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const (
	agentID = "123-456-789"
)

type Agent struct {
	ID   string
	tail *gexec.Session
}

func StartVagrant(provider string) (Agent, error) {
	if len(provider) == 0 {
		provider = "virtualbox"
	}
	command := exec.Command(fmt.Sprintf("./setup_%s.sh", provider))
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		return Agent{}, err
	}
	gomega.Eventually(session, 20*time.Minute).Should(gexec.Exit(0))

	// agentTail, err := gexec.Start(exec.Command("bash", "-c", "tail -f service_wrapper.*.log"), ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	// if err != nil {
	// return Agent{}, err
	// }

	// gomega.Eventually(agentTail, 30*time.Second).Should(gbytes.Say("Subscribing to agent"))
	return Agent{
		ID: agentID,
	}, nil
}

func (a Agent) Stop() {
	// vagrant destroy
}
