package windows_test

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/apcera/nats"
	"github.com/cloudfoundry/bosh-agent/agent/action"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	agentGuid = "123-456-789"
	agentID   = "agent." + agentGuid
	senderID  = "director.987-654-321"
)

func natsURI() string {
	natsURL := "nats://172.31.180.3:4222"
	vagrantProvider := os.Getenv("VAGRANT_PROVIDER")
	if vagrantProvider == "aws" {
		natsURL = fmt.Sprintf("nats://%s:4222", os.Getenv("NATS_ELASTIC_IP"))
	}
	return natsURL
}

var _ = Describe("An Agent running on Windows", func() {
	It("responds to 'ping' message over NATS", func() {
		message := fmt.Sprintf(`{"method":"ping","arguments":[],"reply_to":"%s"}`, senderID)

		testPing := func() string {
			nc, err := nats.Connect(natsURI())
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

	It("responds to 'get_state' message over NATS", func() {
		getStateSpecAgentId := func() string {
			nc, err := nats.Connect(natsURI())
			if err != nil {
				Fail(fmt.Sprintf("Could not connect to NATS. Error is: %s", err.Error()))
			}
			defer nc.Close()

			sub, err := nc.SubscribeSync(senderID)

			message := fmt.Sprintf(`{"method":"get_state","arguments":[],"reply_to":"%s"}`, senderID)
			if err := nc.Publish(agentID, []byte(message)); err != nil {
				Fail(fmt.Sprintf("Could not publish message: '%s' to agent id: '%s' to the NATS server.\nError is: %v\n", message, agentID, err))
			}

			receivedMessage, err := sub.NextMsg(5 * time.Second)
			if err != nil {
				return err.Error()
			}

			response := map[string]action.GetStateV1ApplySpec{}
			json.Unmarshal(receivedMessage.Data, &response)
			return response["value"].AgentID
		}

		Eventually(getStateSpecAgentId, 30*time.Second, 1*time.Second).Should(Equal(agentGuid))
	})
})
