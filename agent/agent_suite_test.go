package agent_test

import (
	"github.com/cloudfoundry/bosh-agent/agent"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"time"

	"testing"
)

func TestAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent Suite")
}

var _ = BeforeSuite(func() {
	agent.HeartbeatRetryInterval = 1 * time.Millisecond
})
