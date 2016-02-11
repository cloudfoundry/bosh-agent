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
	agentGUID = "123-456-789"
	agentID   = "agent." + agentGUID
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

	It("can run a release job", func() {
		natsClient := NewNatsClient()
		err := natsClient.Setup()
		Expect(err).NotTo(HaveOccurred())
		defer natsClient.Cleanup()

		templateID, renderedTemplateArchiveID, err := UploadJob()
		Expect(err).NotTo(HaveOccurred())

		err = natsClient.PrepareJob(templateID, renderedTemplateArchiveID)
		Expect(err).NotTo(HaveOccurred())

		runStartResponse, err := natsClient.RunStart()
		Expect(err).NotTo(HaveOccurred())

		Expect(runStartResponse["value"]).To(Equal("started"))

		// message := fmt.Sprintf(`{"method":"get_state","arguments":[],"reply_to":"%s"}`, senderID)
		// rawResponse, err := natsClient.SendRawMessage(message)
		// Expect(err).NotTo(HaveOccurred())

		// getStateResponse := map[string]action.GetStateV1ApplySpec{}
		// err = json.Unmarshal(rawResponse, &getStateResponse)
		// Expect(err).NotTo(HaveOccurred())

		// Expect(getStateResponse["value"].JobState).To(Equal("running"))
	})
})
