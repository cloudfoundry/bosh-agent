package windows_test

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/apcera/nats"
	"github.com/cloudfoundry/bosh-agent/agent/action"

	. "github.com/onsi/gomega"
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
