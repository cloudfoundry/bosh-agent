package agentpassword_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAgentPassword(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent Password Suite")
}
