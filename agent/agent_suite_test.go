package agent_test

import (
	"time"

	"github.com/cloudfoundry/bosh-agent/v2/agent"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent Suite")
}

var _ = BeforeSuite(func() {
	agent.HeartbeatRetryInterval = 1 * time.Millisecond
})
