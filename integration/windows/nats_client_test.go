package windows_test

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/apcera/nats"
	"github.com/cloudfoundry/bosh-agent/agent/action"

	. "github.com/onsi/gomega"
)

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
                        "name": "say-hello",
						"blobstore_id": "%s",
						"sha1": "eb9bebdb1f11494b27440ec6ccbefba00e713cd9"
                    }
                ]
            },
            "packages": {},
            "rendered_templates_archive": {
                "blobstore_id": "%s",
                "sha1": "6760d464064ee036db9898c736ff71c6d4457792"
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
					"sha1": "6760d464064ee036db9898c736ff71c6d4457792"
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
)

type natsClient struct {
	nc  *nats.Conn
	sub *nats.Subscription
}

func NewNatsClient() *natsClient {
	return &natsClient{}
}

func (n *natsClient) Setup() error {
	var err error
	n.nc, err = nats.Connect(natsURI())
	if err != nil {
		return err
	}

	n.sub, err = n.nc.SubscribeSync(senderID)
	return err
}

func (n *natsClient) Cleanup() {
	_, err := n.RunStop()
	Expect(err).NotTo(HaveOccurred())

	n.nc.Close()
}

func (n *natsClient) PrepareJob(templateID, renderedTemplateArchiveID string) error {
	message := fmt.Sprintf(prepareTemplate, templateID, renderedTemplateArchiveID, senderID)
	prepareResponse, err := n.SendMessage(message)
	if err != nil {
		return err
	}

	check := n.checkStatus(prepareResponse["value"]["agent_task_id"])
	Eventually(check, 30*time.Second, 1*time.Second).Should(Equal("prepared"))

	message = fmt.Sprintf(applyTemplate, templateID, renderedTemplateArchiveID, senderID)
	applyResponse, err := n.SendMessage(message)
	if err != nil {
		return err
	}

	check = n.checkStatus(applyResponse["value"]["agent_task_id"])
	Eventually(check, 30*time.Second, 1*time.Second).Should(Equal("applied"))

	return nil
}

func (n *natsClient) RunStart() (map[string]string, error) {
	message := fmt.Sprintf(startTemplate, senderID)
	rawResponse, err := n.SendRawMessage(message)
	if err != nil {
		return map[string]string{}, err
	}

	response := map[string]string{}
	err = json.Unmarshal(rawResponse, &response)
	return response, err
}

func (n *natsClient) RunStop() (map[string]map[string]string, error) {
	message := fmt.Sprintf(stopTemplate, senderID)
	rawResponse, err := n.SendRawMessage(message)
	if err != nil {
		return map[string]map[string]string{}, err
	}

	response := map[string]map[string]string{}

	err = json.Unmarshal(rawResponse, &response)
	return response, err
}

func (n *natsClient) RunErrand() (map[string]map[string]string, error) {
	message := fmt.Sprintf(errandTemplate, senderID)
	return n.SendMessage(message)
}

func (n *natsClient) RunFetchLogs() (map[string]map[string]string, error) {
	message := fmt.Sprintf(fetchLogsTemplate, senderID)
	return n.SendMessage(message)
}

func (n *natsClient) CheckFetchLogsStatus(taskID string) func() (map[string]string, error) {
	return func() (map[string]string, error) {
		var result map[string]map[string]string
		valueResponse, err := n.getTask(taskID)
		if err != nil {
			return map[string]string{}, err
		}

		err = json.Unmarshal(valueResponse, &result)
		if err != nil {
			return map[string]string{}, err
		}

		return result["value"], nil
	}
}

func (n *natsClient) CheckErrandResultStatus(taskID string) func() (action.ErrandResult, error) {
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

func (n *natsClient) SendRawMessage(message string) ([]byte, error) {
	err := n.nc.Publish(agentID, []byte(message))
	if err != nil {
		return nil, err
	}

	raw, err := n.sub.NextMsg(5 * time.Second)
	if err != nil {
		return nil, err
	}

	return raw.Data, nil
}

func (n *natsClient) SendMessage(message string) (map[string]map[string]string, error) {
	rawMessage, err := n.SendRawMessage(message)
	if err != nil {
		return nil, err
	}

	response := map[string]map[string]string{}
	err = json.Unmarshal(rawMessage, &response)
	return response, err
}

func (n *natsClient) getTask(taskID string) ([]byte, error) {
	message := fmt.Sprintf(`{"method": "get_task", "arguments": ["%s"], "reply_to": "%s"}`, taskID, senderID)
	return n.SendRawMessage(message)
}

func (n *natsClient) checkStatus(taskID string) func() (string, error) {
	return func() (string, error) {
		var result map[string]string
		valueResponse, err := n.getTask(taskID)
		if err != nil {
			return "", err
		}

		err = json.Unmarshal(valueResponse, &result)
		if err != nil {
			return "", err
		}

		return result["value"], nil
	}
}
