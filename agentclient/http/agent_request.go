package http

import (
	"encoding/json"
	"io"
	"net/http"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/httpclient"
)

type AgentRequestMessage struct {
	Method    string        `json:"method"`
	Arguments []interface{} `json:"arguments"`
	ReplyTo   string        `json:"reply_to"`
}

type agentRequest struct {
	directorID string
	endpoint   string
	httpClient *httpclient.HTTPClient
}

func (r agentRequest) Send(method string, arguments []interface{}, response Response) error {
	postBody := AgentRequestMessage{
		Method:    method,
		Arguments: arguments,
		ReplyTo:   r.directorID,
	}

	agentRequestJSON, err := json.Marshal(postBody)
	if err != nil {
		return bosherr.WrapError(err, "Marshaling agent request")
	}

	httpResponse, err := r.httpClient.PostCustomized(r.endpoint, agentRequestJSON, func(r *http.Request) {
		r.Header["Content-type"] = []string{"application/json"}
	})

	if err != nil {
		return bosherr.WrapErrorf(err, "Performing request to agent")
	}
	defer func() {
		_ = httpResponse.Body.Close() //nolint:errcheck
	}()

	if httpResponse.StatusCode != http.StatusOK {
		return bosherr.Errorf("Agent responded with non-successful status code: %d", httpResponse.StatusCode)
	}

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return bosherr.WrapError(err, "Reading agent response")
	}

	err = response.Unmarshal(responseBody)
	if err != nil {
		return bosherr.WrapError(err, "Unmarshaling agent response")
	}

	err = response.ServerError()
	if err != nil {
		return err
	}

	return nil
}
