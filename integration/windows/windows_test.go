package windows_test

import (
	"fmt"
	"time"

	"github.com/apcera/nats"
	"github.com/cloudfoundry/bosh-agent/integration/windows/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("An Agent running on Windows", func() {
	It("responds to 'ping' message over NATS", func() {
		nc, err := nats.Connect(nats.DefaultURL)

		if err != nil {
			Fail(fmt.Sprintf("Could not connect to the nats server at: '%s'.\nError is: %s\n", nats.DefaultURL, err))
		}

		defer nc.Close()

		agent, err := utils.StartAgent()
		defer agent.Stop()
		if err != nil {
			Fail(fmt.Sprintln("Could not build the bosh-agent project.\nError is:", err))
		}

		agentID := "agent." + agent.ID
		senderID := "director.987-654-321"
		message := fmt.Sprintf(`{"method":"ping","arguments":[],"reply_to":"%s"}`, senderID)

		sub, err := nc.SubscribeSync(senderID)

		if err := nc.Publish(agentID, []byte(message)); err != nil {
			Fail(fmt.Sprintf("Could not publish message: '%s' to agent id: '%s' to the NATS server.\nError is: %v\n", message, agentID, err))
		}

		receivedMessage, err := sub.NextMsg(5 * time.Second)
		if err != nil {
			Fail(fmt.Sprintf("Agent has not responded.\nError is: %v\n", err))
		}

		Expect(string(receivedMessage.Data)).To(Equal(`{"value":"pong"}`))
	})
})
