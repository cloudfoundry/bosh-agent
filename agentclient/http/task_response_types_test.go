package http_test

import (
	"testing"

	agenthttp "github.com/cloudfoundry/bosh-agent/v2/agentclient/http"
)

func TestTaskResponseTypes(t *testing.T) {
	for _, value := range []interface{}{nil, 1.0, []interface{}{}, map[string]interface{}{}} {
		response := agenthttp.TaskResponse{Value: map[string]interface{}{"agent_task_id": value}}
		if _, err := response.TaskID(); err == nil {
			t.Fatalf("accepted task ID %T", value)
		}
		response = agenthttp.TaskResponse{Value: map[string]interface{}{"agent_task_id": "task", "state": value}}
		if _, err := response.TaskState(); err == nil {
			t.Fatalf("accepted task state %T", value)
		}
	}
}
