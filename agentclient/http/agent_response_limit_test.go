package http

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type endlessAgentBody struct {
	count  int
	closed bool
}

func (b *endlessAgentBody) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	b.count += len(p)
	return len(p), nil
}
func (b *endlessAgentBody) Close() error { b.closed = true; return nil }

type fixedAgentClient struct{ body *endlessAgentBody }

func (c fixedAgentClient) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: c.body}, nil
}
func TestAgentResponseSizeIsBounded(t *testing.T) {
	body := &endlessAgentBody{}
	request := agentRequest{endpoint: "http://example.test/agent", httpClient: httpclient.NewHTTPClient(fixedAgentClient{body}, boshlog.NewLogger(boshlog.LevelNone))}
	err := request.Send("ping", nil, &SimpleTaskResponse{})
	if err == nil || !strings.Contains(err.Error(), "exceeds size limit") {
		t.Fatalf("unexpected response result: %v", err)
	}
	if body.count > maxAgentResponseBytes+1 {
		t.Fatalf("read too much data: %d", body.count)
	}
	if !body.closed {
		t.Fatal("response body left open")
	}
}
