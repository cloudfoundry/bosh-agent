package windows_test

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/apcera/nats"
	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/integration/windows/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	agentGuid       = "123-456-789"
	agentID         = "agent." + agentGuid
	senderID        = "director.987-654-321"
	prepareTemplate = `{
    "arguments": [
        {
            "deployment": "test",
            "job": {
                "name": "test-job",
                "template": "test-job",
                "templates": [
                    {
                        "name": "say-hello"
                    }
                ]
            },
            "packages": {},
            "rendered_templates_archive": {
                "blobstore_id": "%[1]s",
                "sha1": "989f9a99678b253eb039a2faec092aa09038e053"
            }
        }
    ],
    "method": "prepare",
    "reply_to": "%[2]s"
}`
)

func natsURI() string {
	natsURL := "nats://172.31.180.3:4222"
	vagrantProvider := os.Getenv("VAGRANT_PROVIDER")
	if vagrantProvider == "aws" {
		natsURL = fmt.Sprintf("nats://%s:4222", os.Getenv("NATS_ELASTIC_IP"))
	}
	return natsURL
}

func blobstoreURI() string {
	natsURL := "http://172.31.180.3:25250"
	vagrantProvider := os.Getenv("VAGRANT_PROVIDER")
	if vagrantProvider == "aws" {
		// do something
	}
	return natsURL
}

func testPing() string {
	message := fmt.Sprintf(`{"method":"ping","arguments":[],"reply_to":"%s"}`, senderID)
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

var _ = Describe("An Agent running on Windows", func() {
	BeforeEach(func() {
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

	It("can run a prepare action", func() {
		blobstore := utils.NewBlobstore(blobstoreURI())

		blobID, err := blobstore.Create("fixtures/job.tar")
		Expect(err).NotTo(HaveOccurred())

		nc, err := nats.Connect(natsURI())
		Expect(err).NotTo(HaveOccurred())
		defer nc.Close()

		prepareMessage := fmt.Sprintf(prepareTemplate, blobID, senderID)
		err = nc.Publish(agentID, []byte(prepareMessage))
		Expect(err).NotTo(HaveOccurred())

		sub, err := nc.SubscribeSync(senderID)
		raw, err := sub.NextMsg(5 * time.Second)
		Expect(err).NotTo(HaveOccurred())

		response := map[string]map[string]string{}
		json.Unmarshal(raw.Data, &response)

		callthatfunc := func() string {
			getTaskMessage := fmt.Sprintf(`{"method": "get_task", "arguments": ["%s"], "reply_to": "%s"}`, response["value"]["agent_task_id"], senderID)
			if err := nc.Publish(agentID, []byte(getTaskMessage)); err != nil {
				Fail(fmt.Sprintf("Could not publish message: '%s' to agent id: '%s' to the NATS server.\nError is: %v\n", getTaskMessage, agentID, err))
			}
			receivedMessage, err := sub.NextMsg(5 * time.Second)
			if err != nil {
				return err.Error()
			}
			GinkgoWriter.Write(receivedMessage.Data)
			GinkgoWriter.Write([]byte{'\n'})
			return string(receivedMessage.Data)
		}
		Eventually(callthatfunc, 30*time.Second, 1*time.Second).Should(Equal(`{"value":"prepared"}`))
	})
})
