package integrationagentclient

import (
	"encoding/json"
	"time"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/agentclient/http"
	"github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type IntegrationAgentClient struct {
	*http.AgentClient
}

func NewIntegrationAgentClient(
	endpoint string,
	directorID string,
	getTaskDelay time.Duration,
	toleratedErrorCount int,
	httpClient *httpclient.HTTPClient,
	logger boshlog.Logger,
) *IntegrationAgentClient {
	return &IntegrationAgentClient{
		AgentClient: http.NewAgentClient(endpoint, directorID, getTaskDelay, toleratedErrorCount, httpClient, logger).(*http.AgentClient),
	}
}

type exception struct {
	Message string
}

type SSHResponse struct {
	action.SSHResult
	Exception *exception
}

func (r *SSHResponse) ServerError() error {
	if r.Exception != nil {
		return bosherr.Errorf("Agent responded with error: %s", r.Exception.Message)
	}
	return nil
}

func (r *SSHResponse) Unmarshal(message []byte) error {
	return json.Unmarshal(message, r)
}

func (c *IntegrationAgentClient) FetchLogs(logType string, filters []string) (map[string]interface{}, error) {
	responseRaw, err := c.SendAsyncTaskMessage("fetch_logs", []interface{}{logType, filters})
	if err != nil {
		return nil, bosherr.WrapError(err, "Fetching logs from agent")
	}
	responseValue, ok := responseRaw.(map[string]interface{})
	if !ok {
		return nil, bosherr.Errorf("Unable to parse fetch_logs response value: %#v", responseRaw)
	}
	if err != nil {
		return nil, bosherr.WrapError(err, "Sending 'fetch_logs' to the agent")
	}

	return responseValue, err
}

func (c *IntegrationAgentClient) SSH(cmd string, params action.SSHParams) error {
	err := c.AgentRequest.Send("ssh", []interface{}{cmd, params}, &SSHResponse{})
	if err != nil {
		return bosherr.WrapError(err, "Sending 'ssh' to the agent")
	}

	return nil
}

func (c *IntegrationAgentClient) UpdateSettings(settings settings.UpdateSettings) error {
	_, err := c.SendAsyncTaskMessage("update_settings", []interface{}{settings})
	return err
}
