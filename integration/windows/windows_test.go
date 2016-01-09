package windows_test

import (
	"fmt"
	"time"

	. "github.com/cloudfoundry/bosh-agent/integration/windows"
	"github.com/nats-io/nats"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("An Agent running on Windows", func() {
	It("responds to 'ping' message over NATS", func() {
		nc, err := nats.Connect(nats.DefaultURL)

		if err != nil {
			Fail(fmt.Sprintf("Could NOT connect to the nats server at: '%s', bruh.\nError is: %s\n", nats.DefaultURL, err))
		}

		defer nc.Close()

		agent, err := StartAgent()
		if err != nil {
			Fail(fmt.Sprintln("Could NOT build the bosh-agent project, bruh.\nError is:", err))
		}

		agentId := "agent." + agent.Id
		senderId := "director.987-654-321"
		message := fmt.Sprintf(`{"method":"ping","arguments":[],"reply_to":"%s"}`, senderId)

		sub, err := nc.SubscribeSync(senderId)

		if err := nc.Publish(agentId, []byte(message)); err != nil {
			Fail(fmt.Sprintf("Could NOT publish message: '%s' to agent id: '%s' to the NATS server, bruh.\nError is: %v\n", message, agentId, err))
		}

		receivedMessage, err := sub.NextMsg(5 * time.Second)
		if err != nil {
			Fail(fmt.Sprintf("Could NOT receive the next NATS message in the channel, bruh.\nError is: %v\n", err))
		}

		Expect(string(receivedMessage.Data)).To(Equal(`{"value":"pong"}`))
	})
})
