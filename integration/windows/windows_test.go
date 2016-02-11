package windows_test

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/integration/windows/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	agentGUID       = "123-456-789"
	agentID         = "agent." + agentGUID
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
                "sha1": "46a41b2acc4134444d124e949f719a312ccdf806"
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
					"sha1": "46a41b2acc4134444d124e949f719a312ccdf806"
			}
        }
    ],
    "method": "apply",
    "protocol": 2,
    "reply_to": "%[3]s"
}
	`

	fetchLogsTemplate = `
{
    "arguments": [
        "job",
        null
    ],
    "method": "fetch_logs",
    "protocol": 2,
    "reply_to": "%s"
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

var _ = Describe("An Agent running on Windows", func() {
	BeforeEach(func() {
		message := fmt.Sprintf(`{"method":"ping","arguments":[],"reply_to":"%s"}`, senderID)
		natsClient := NewNatsClient()
		err := natsClient.Setup()
		Expect(err).NotTo(HaveOccurred())
		defer natsClient.Cleanup()

		testPing := func() (string, error) {
			response, err := natsClient.SendRawMessage(message)
			return string(response), err
		}

		Eventually(testPing, 30*time.Second, 1*time.Second).Should(Equal(`{"value":"pong"}`))
	})

	It("responds to 'get_state' message over NATS", func() {
		getStateSpecAgentID := func() string {
			natsClient := NewNatsClient()
			err := natsClient.Setup()
			Expect(err).NotTo(HaveOccurred())
			defer natsClient.Cleanup()

			message := fmt.Sprintf(`{"method":"get_state","arguments":[],"reply_to":"%s"}`, senderID)
			rawResponse, err := natsClient.SendRawMessage(message)
			Expect(err).NotTo(HaveOccurred())

			response := map[string]action.GetStateV1ApplySpec{}
			err = json.Unmarshal(rawResponse, &response)
			Expect(err).NotTo(HaveOccurred())

			return response["value"].AgentID
		}

		Eventually(getStateSpecAgentID, 30*time.Second, 1*time.Second).Should(Equal(agentGUID))
	})

	It("can run a run_errand action", func() {
		natsClient := NewNatsClient()
		err := natsClient.Setup()
		Expect(err).NotTo(HaveOccurred())
		defer natsClient.Cleanup()

		templateID, renderedTemplateArchiveID, err := UploadJob()
		Expect(err).NotTo(HaveOccurred())

		err = natsClient.PrepareJob(templateID, renderedTemplateArchiveID)
		Expect(err).NotTo(HaveOccurred())

		runErrandResponse, err := natsClient.RunErrand()
		Expect(err).NotTo(HaveOccurred())

		runErrandCheck := natsClient.CheckErrandResultStatus(runErrandResponse["value"]["agent_task_id"])
		Eventually(runErrandCheck, 30*time.Second, 1*time.Second).Should(Equal(action.ErrandResult{
			Stdout:     "hello world\r\n",
			ExitStatus: 0,
		}))

		fetchLogsResponse, err := natsClient.RunFetchLogs()
		Expect(err).NotTo(HaveOccurred())
		fetchLogsCheck := natsClient.CheckFetchLogsStatus(fetchLogsResponse["value"]["agent_task_id"])
		Eventually(fetchLogsCheck, 30*time.Second, 1*time.Second).Should(HaveKey("blobstore_id"))
	})
})
