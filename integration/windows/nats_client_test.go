package windows_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"text/template"
	"time"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	boshalert "github.com/cloudfoundry/bosh-agent/agent/alert"
	"github.com/cloudfoundry/bosh-agent/agentclient/http"
	"github.com/cloudfoundry/bosh-agent/integration/windows/utils"
	boshfileutil "github.com/cloudfoundry/bosh-utils/fileutil"
	"github.com/nats-io/nats"

	. "github.com/onsi/gomega"
)

type PrepareTemplateConfig struct {
	JobName                             string
	TemplateBlobstoreID                 string
	RenderedTemplatesArchiveBlobstoreID string
	RenderedTemplatesArchiveSHA1        string
	ReplyTo                             string
}

const (
	prepareTemplate = `{
    "arguments": [
        {
            "deployment": "test",
            "job": {
                "name": "test-job",
                "template": "test-job",
                "templates": [
                    {
                        "name": "{{ .JobName }}",
						"blobstore_id": "{{ .TemplateBlobstoreID }}",
						"sha1": "eb9bebdb1f11494b27440ec6ccbefba00e713cd9"
                    }
                ]
            },
            "packages": {},
            "rendered_templates_archive": {
                "blobstore_id": "{{ .RenderedTemplatesArchiveBlobstoreID }}",
                "sha1": "{{ .RenderedTemplatesArchiveSHA1 }}"
            }
        }
    ],
    "method": "prepare",
    "reply_to": "{{ .ReplyTo }}"
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
                "blobstore_id": "{{ .TemplateBlobstoreID }}",
                "name": "{{ .JobName }}",
                "sha1": "eb6e6c8bd1b1bc3dd91c741ec5c628b61a4d8f1d",
                "template": "say-hello",
                "templates": [
                    {
                        "blobstore_id": "{{ .TemplateBlobstoreID }}",
                        "name": "{{ .JobName }}",
                        "sha1": "eb6e6c8bd1b1bc3dd91c741ec5c628b61a4d8f1d",
                        "version": "8fe0a4982b28ffe4e59d7c1e573c4f30a526770d"
                    }
                ],
                "version": "8fe0a4982b28ffe4e59d7c1e573c4f30a526770d"
            },
            "networks": {},
			"rendered_templates_archive": {
					"blobstore_id": "{{ .RenderedTemplatesArchiveBlobstoreID }}",
					"sha1": "{{ .RenderedTemplatesArchiveSHA1 }}"
			}
        }
    ],
    "method": "apply",
    "protocol": 2,
    "reply_to": "{{ .ReplyTo }}"
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

	startTemplate = `
{
	"arguments":[],
	"method":"start",
	"protocol":2,
	"reply_to":"%s"
}
	`
	stopTemplate = `
{
	"protocol":2,
	"method":"stop",
	"arguments":[],
	"reply_to":"%s"
}
	`

	drainTemplate = `
{
	"protocol":2,
	"method":"drain",
	"arguments":[
	  "update",
	  {}
	],
	"reply_to":"%s"
}
	`
	runScriptTemplate = `
{
	"protocol":2,
	"method":"run_script",
	"arguments":[
	  "%s",
	  {}
	],
	"reply_to":"%s"
}
	`
)

type NatsClient struct {
	nc       *nats.Conn
	sub      *nats.Subscription
	alertSub *nats.Subscription

	compressor      boshfileutil.Compressor
	blobstoreClient utils.BlobClient

	renderedTemplatesArchivesSha1 map[string]string
}

func NewNatsClient(
	compressor boshfileutil.Compressor,
	blobstoreClient utils.BlobClient,
) *NatsClient {
	return &NatsClient{
		compressor:      compressor,
		blobstoreClient: blobstoreClient,
		renderedTemplatesArchivesSha1: map[string]string{
			"say-hello":       "8d62be87451e2ac3b5b3e736d210176274c95ec9",
			"unmonitor-hello": "4ff9960a1d594743c498141cdbd611b93262e78c",
		},
	}
}

func (n *NatsClient) Setup() error {
	var err error
	n.nc, err = nats.Connect(natsURI())
	if err != nil {
		return err
	}

	n.sub, err = n.nc.SubscribeSync(senderID)
	n.alertSub, err = n.nc.SubscribeSync("hm.agent.alert." + agentGUID)
	return err
}

func (n *NatsClient) Cleanup() {
	err := n.RunStop()
	Expect(err).NotTo(HaveOccurred())

	n.nc.Close()
}

func (n *NatsClient) PrepareJob(jobName string) {
	templateID, renderedTemplateArchiveID, err := n.uploadJob(jobName)
	Expect(err).NotTo(HaveOccurred())

	sha1 := n.renderedTemplatesArchivesSha1[jobName]

	prepareTemplateConfig := PrepareTemplateConfig{
		JobName:                             jobName,
		TemplateBlobstoreID:                 templateID,
		RenderedTemplatesArchiveBlobstoreID: renderedTemplateArchiveID,
		RenderedTemplatesArchiveSHA1:        sha1,
		ReplyTo: senderID,
	}

	buffer := bytes.NewBuffer([]byte{})
	t := template.Must(template.New("prepare").Parse(prepareTemplate))
	err = t.Execute(buffer, prepareTemplateConfig)
	Expect(err).NotTo(HaveOccurred())
	prepareResponse, err := n.SendMessage(buffer.String())
	Expect(err).NotTo(HaveOccurred())

	check := n.checkStatus(prepareResponse["value"]["agent_task_id"])
	Eventually(check, 30*time.Second, 1*time.Second).Should(Equal("finished"))

	buffer.Reset()
	t = template.Must(template.New("apply").Parse(applyTemplate))
	err = t.Execute(buffer, prepareTemplateConfig)
	Expect(err).NotTo(HaveOccurred())
	applyResponse, err := n.SendMessage(buffer.String())
	Expect(err).NotTo(HaveOccurred())

	check = n.checkStatus(applyResponse["value"]["agent_task_id"])
	Eventually(check, 30*time.Second, 1*time.Second).Should(Equal("finished"))
}

func (n *NatsClient) RunDrain() error {
	message := fmt.Sprintf(drainTemplate, senderID)
	drainResponse, err := n.SendMessage(message)
	if err != nil {
		return err
	}

	check := n.checkDrain(drainResponse["value"]["agent_task_id"])
	Eventually(check, 30*time.Second, 1*time.Second).Should(Equal(0))

	return nil
}

func (n *NatsClient) RunScript(scriptName string) error {
	message := fmt.Sprintf(runScriptTemplate, scriptName, senderID)
	response, err := n.SendMessage(message)
	if err != nil {
		return err
	}

	check := func() (map[string]string, error) {
		var result map[string]map[string]string
		valueResponse, err := n.getTask(response["value"]["agent_task_id"])
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(valueResponse, &result)
		if err != nil {
			return nil, err
		}

		return result["value"], nil
	}

	// run_script commands just return {} when they're done
	Eventually(check, 30*time.Second, 1*time.Second).Should(Equal(map[string]string{}))

	return nil
}

func (n *NatsClient) RunStart() (map[string]string, error) {
	message := fmt.Sprintf(startTemplate, senderID)
	rawResponse, err := n.SendRawMessageWithTimeout(message, time.Minute)
	if err != nil {
		return map[string]string{}, err
	}

	response := map[string]string{}
	err = json.Unmarshal(rawResponse, &response)
	return response, err
}

func (n *NatsClient) GetState() action.GetStateV1ApplySpec {
	message := fmt.Sprintf(`{"method":"get_state","arguments":[],"reply_to":"%s"}`, senderID)
	rawResponse, err := n.SendRawMessage(message)
	Expect(err).NotTo(HaveOccurred())

	getStateResponse := map[string]action.GetStateV1ApplySpec{}
	err = json.Unmarshal(rawResponse, &getStateResponse)
	Expect(err).NotTo(HaveOccurred())

	return getStateResponse["value"]
}

func (n *NatsClient) RunStop() error {
	message := fmt.Sprintf(stopTemplate, senderID)
	rawResponse, err := n.SendRawMessage(message)
	if err != nil {
		return err
	}

	response := map[string]map[string]string{}

	err = json.Unmarshal(rawResponse, &response)
	if err != nil {
		return err
	}

	check := n.checkStatus(response["value"]["agent_task_id"])
	Eventually(check, 30*time.Second, 1*time.Second).Should(Equal("finished"))

	return nil
}

func (n *NatsClient) RunErrand() (map[string]map[string]string, error) {
	message := fmt.Sprintf(errandTemplate, senderID)
	return n.SendMessage(message)
}

func (n *NatsClient) FetchLogs(destinationDir string) {
	message := fmt.Sprintf(fetchLogsTemplate, senderID)
	fetchLogsResponse, err := n.SendMessage(message)
	var fetchLogsResult map[string]string

	fetchLogsCheckFunc := func() (map[string]string, error) {
		var err error
		var taskResult map[string]map[string]string

		valueResponse, err := n.getTask(fetchLogsResponse["value"]["agent_task_id"])
		if err != nil {
			return map[string]string{}, err
		}

		err = json.Unmarshal(valueResponse, &taskResult)
		if err != nil {
			return map[string]string{}, err
		}

		fetchLogsResult = taskResult["value"]

		return fetchLogsResult, nil
	}

	Eventually(fetchLogsCheckFunc, 30*time.Second, 1*time.Second).Should(HaveKey("blobstore_id"))

	fetchedLogFile := filepath.Join(destinationDir, "log.tgz")
	err = n.blobstoreClient.Get(fetchLogsResult["blobstore_id"], fetchedLogFile)
	Expect(err).NotTo(HaveOccurred())

	err = n.compressor.DecompressFileToDir(fetchedLogFile, destinationDir, boshfileutil.CompressorOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func (n *NatsClient) CheckErrandResultStatus(taskID string) func() (action.ErrandResult, error) {
	return func() (action.ErrandResult, error) {
		var result map[string]action.ErrandResult
		valueResponse, err := n.getTask(taskID)
		if err != nil {
			return action.ErrandResult{}, err
		}

		err = json.Unmarshal(valueResponse, &result)
		if err != nil {
			return action.ErrandResult{}, err
		}

		return result["value"], nil
	}
}

func (n *NatsClient) SendRawMessageWithTimeout(message string, timeout time.Duration) ([]byte, error) {
	err := n.nc.Publish(agentID, []byte(message))
	if err != nil {
		return nil, err
	}

	raw, err := n.sub.NextMsg(timeout)
	if err != nil {
		return nil, err
	}

	return raw.Data, nil
}

func (n *NatsClient) SendRawMessage(message string) ([]byte, error) {
	return n.SendRawMessageWithTimeout(message, 5*time.Second)
}

func (n *NatsClient) SendMessage(message string) (map[string]map[string]string, error) {
	rawMessage, err := n.SendRawMessage(message)
	if err != nil {
		return nil, err
	}

	response := map[string]map[string]string{}
	err = json.Unmarshal(rawMessage, &response)
	return response, err
}

func (n *NatsClient) GetNextAlert(timeout time.Duration) (*boshalert.Alert, error) {
	raw, err := n.alertSub.NextMsg(timeout)
	if err != nil {
		return nil, err
	}
	var alert boshalert.Alert
	err = json.Unmarshal(raw.Data, &alert)
	return &alert, err
}

func (n *NatsClient) getTask(taskID string) ([]byte, error) {
	message := fmt.Sprintf(`{"method": "get_task", "arguments": ["%s"], "reply_to": "%s"}`, taskID, senderID)
	return n.SendRawMessage(message)
}

func (n *NatsClient) checkStatus(taskID string) func() (string, error) {
	return func() (string, error) {
		var result http.TaskResponse
		valueResponse, err := n.getTask(taskID)
		if err != nil {
			return "", err
		}

		err = json.Unmarshal(valueResponse, &result)
		if err != nil {
			return "", err
		}

		if err = result.ServerError(); err != nil {
			return "", err
		}

		return result.TaskState()
	}
}

func (n *NatsClient) checkDrain(taskID string) func() (int, error) {
	return func() (int, error) {
		var result map[string]int
		valueResponse, err := n.getTask(taskID)
		if err != nil {
			return -1, err
		}

		err = json.Unmarshal(valueResponse, &result)
		if err != nil {
			return -1, err
		}

		return result["value"], nil
	}
}

func (n *NatsClient) uploadJob(jobName string) (templateID, renderedTemplateArchiveID string, err error) {
	renderedTemplateArchiveID, err = n.blobstoreClient.Create(fmt.Sprintf("fixtures/rendered_templates_archives/%s.tar", jobName))
	if err != nil {
		return
	}
	templateID, err = n.blobstoreClient.Create("fixtures/template.tar")
	if err != nil {
		return
	}
	return
}
