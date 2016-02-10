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
                        "name": "say-hello",
												"blobstore_id": "%s",
												"sha1": "eb9bebdb1f11494b27440ec6ccbefba00e713cd9"
                    }
                ]
            },
            "packages": {},
            "rendered_templates_archive": {
                "blobstore_id": "%s",
                "sha1": "80848728c3e2e27027ef44d0e2448d2f314567be"
            }
        }
    ],
    "method": "prepare",
    "reply_to": "%s"
}`
	errandTemplate = `
	{"protocol":2,"method":"run_errand","arguments":[],"reply_to":"%s"}
	`
	applyTemplate = `
{
    "arguments": [
        {
            "configuration_hash": "foo",
            "deployment": "hello-world-windows-deployment",
            "id": "62236318-6632-4318-94c7-c3dd6e8e5698",
            "index": 0,
            "job": {
                "blobstore_id": "%[1]s",
                "name": "say-hello",
                "sha1": "eb6e6c8bd1b1bc3dd91c741ec5c628b61a4d8f1d",
                "template": "say-hello",
                "templates": [
                    {
                        "blobstore_id": "%[1]s",
                        "name": "say-hello",
                        "sha1": "eb6e6c8bd1b1bc3dd91c741ec5c628b61a4d8f1d",
                        "version": "8fe0a4982b28ffe4e59d7c1e573c4f30a526770d"
                    }
                ],
                "version": "8fe0a4982b28ffe4e59d7c1e573c4f30a526770d"
            },
            "networks": {},
						"rendered_templates_archive": {
								"blobstore_id": "%[2]s",
								"sha1": "80848728c3e2e27027ef44d0e2448d2f314567be"
						}
        }
    ],
    "method": "apply",
    "protocol": 2,
    "reply_to": "%[3]s"
}
	`
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
	blobstoreURI := "http://172.31.180.3:25250"
	vagrantProvider := os.Getenv("VAGRANT_PROVIDER")
	if vagrantProvider == "aws" {
		blobstoreURI = fmt.Sprintf("http://%s:25250", os.Getenv("NATS_ELASTIC_IP"))
	}
	return blobstoreURI
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

func UploadJob() (templateID, renderedTemplateArchiveID string, err error) {
	blobstore := utils.NewBlobstore(blobstoreURI())

	renderedTemplateArchiveID, err = blobstore.Create("fixtures/rendered_templates_archive.tar")
	if err != nil {
		return
	}
	templateID, err = blobstore.Create("fixtures/template.tar")
	if err != nil {
		return
	}
	return
}

func RunPrepare(nc *nats.Conn, sub *nats.Subscription, templateID, renderedTemplateArchiveID string) (map[string]map[string]string, error) {
	prepareMessage := fmt.Sprintf(prepareTemplate, templateID, renderedTemplateArchiveID, senderID)
	err := nc.Publish(agentID, []byte(prepareMessage))
	if err != nil {
		return nil, err
	}

	raw, err := sub.NextMsg(5 * time.Second)
	if err != nil {
		return nil, err
	}

	response := map[string]map[string]string{}
	err = json.Unmarshal(raw.Data, &response)
	return response, err
}

func RunApply(nc *nats.Conn, sub *nats.Subscription, templateID, renderedTemplateArchiveID string) (map[string]map[string]string, error) {
	message := fmt.Sprintf(applyTemplate, templateID, renderedTemplateArchiveID, senderID)
	err := nc.Publish(agentID, []byte(message))
	if err != nil {
		return nil, err
	}

	raw, err := sub.NextMsg(5 * time.Second)
	if err != nil {
		return nil, err
	}

	response := map[string]map[string]string{}
	err = json.Unmarshal(raw.Data, &response)
	return response, err
}

func RunErrand(nc *nats.Conn, sub *nats.Subscription) (map[string]map[string]string, error) {
	message := fmt.Sprintf(errandTemplate, senderID)
	err := nc.Publish(agentID, []byte(message))
	if err != nil {
		return nil, err
	}

	raw, err := sub.NextMsg(5 * time.Second)
	if err != nil {
		return nil, err
	}

	response := map[string]map[string]string{}
	err = json.Unmarshal(raw.Data, &response)
	return response, err
}

func checkStatus(nc *nats.Conn, sub *nats.Subscription, agentTaskId string) func() string {
	return func() string {
		getTaskMessage := fmt.Sprintf(`{"method": "get_task", "arguments": ["%s"], "reply_to": "%s"}`, agentTaskId, senderID)
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

	It("can run a run_errand action", func() {
		nc, err := nats.Connect(natsURI())
		Expect(err).NotTo(HaveOccurred())
		defer nc.Close()

		sub, err := nc.SubscribeSync(senderID)
		Expect(err).NotTo(HaveOccurred())

		templateID, renderedTemplateArchiveID, err := UploadJob()
		Expect(err).NotTo(HaveOccurred())

		prepareResponse, err := RunPrepare(nc, sub, templateID, renderedTemplateArchiveID)
		Expect(err).NotTo(HaveOccurred())

		check := checkStatus(nc, sub, prepareResponse["value"]["agent_task_id"])
		Eventually(check, 30*time.Second, 1*time.Second).Should(Equal(`{"value":"prepared"}`))

		applyResponse, err := RunApply(nc, sub, templateID, renderedTemplateArchiveID)
		Expect(err).NotTo(HaveOccurred())

		check = checkStatus(nc, sub, applyResponse["value"]["agent_task_id"])
		Eventually(check, 30*time.Second, 1*time.Second).Should(Equal(`{"value":"applied"}`))

		response, err := RunErrand(nc, sub)
		Expect(err).NotTo(HaveOccurred())

		checkRunErrand := func() (action.ErrandResult, error) {
			var valueResponse map[string]action.ErrandResult

			getTaskMessage := fmt.Sprintf(`{"method": "get_task", "arguments": ["%s"], "reply_to": "%s"}`, response["value"]["agent_task_id"], senderID)
			if err := nc.Publish(agentID, []byte(getTaskMessage)); err != nil {
				Fail(fmt.Sprintf("Could not publish message: '%s' to agent id: '%s' to the NATS server.\nError is: %v\n", getTaskMessage, agentID, err))
			}
			receivedMessage, err := sub.NextMsg(5 * time.Second)
			if err != nil {
				return action.ErrandResult{}, err
			}
			GinkgoWriter.Write(receivedMessage.Data)
			GinkgoWriter.Write([]byte{'\n'})

			err = json.Unmarshal(receivedMessage.Data, &valueResponse)
			if err != nil {
				return action.ErrandResult{}, err
			}

			return valueResponse["value"], nil
		}

		Eventually(checkRunErrand, 30*time.Second, 1*time.Second).Should(Equal(action.ErrandResult{
			Stdout:     "hello world\r\n",
			ExitStatus: 0,
		}))
	})
})
