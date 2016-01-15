package windows_test

import (
	"fmt"
	"os"
	"time"

	"github.com/apcera/nats"
	"github.com/cloudfoundry/bosh-agent/integration/windows/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("An Agent running on Windows", func() {
	It("responds to 'ping' message over NATS", func() {
		vagrantProvider := os.Getenv("VAGRANT_PROVIDER")

		agent, err := utils.StartVagrant(vagrantProvider)
		if err != nil {
			Fail(fmt.Sprintln("Could not build the bosh-agent project.\nError is:", err))
		}
		defer agent.Stop()

		agentID := "agent." + agent.ID
		senderID := "director.987-654-321"
		message := fmt.Sprintf(`{"method":"ping","arguments":[],"reply_to":"%s"}`, senderID)

		natsURL := "nats://172.31.180.3:4222"
		if vagrantProvider == "aws" {
			natsURL = fmt.Sprintf("nats://%s:4222", os.Getenv("NATS_ELASTIC_IP"))
		}

		testPing := func() string {
			nc, err := nats.Connect(natsURL)
			if err != nil {
				return err.Error()
			}
			defer nc.Close()

			sub, err := nc.SubscribeSync(senderID)

			if err := nc.Publish(agentID, []byte(message)); err != nil {
				Fail(fmt.Sprintf("Could not publish message: '%s' to agent id: '%s' to the NATS server.\nError is: %v\n", message, agentID, err))
			}

			receivedMessage, err := sub.NextMsg(5 * time.Second)
			if err != nil {
				return err.Error()
			}
			return string(receivedMessage.Data)
		}

		Eventually(testPing, 30*time.Second, 1*time.Second).Should(Equal(`{"value":"pong"}`))
	})
})
