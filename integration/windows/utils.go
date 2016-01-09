package windows

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
)

const (
	agentId = "123-456-789"
)

type Agent struct {
	Id string
}

func StartAgent() (Agent, error) {
	fmt.Println("Firing up a vagrant box with your BOSH Agent inside.")

	command := exec.Command("./setup.sh")
	if _, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter); err != nil {
		return Agent{}, err
	}

	fmt.Println("Startup sucesssful. Waiting for provisioning of the box to finish...")
	time.Sleep(20 * time.Second)
	return Agent{Id: agentId}, nil
}
