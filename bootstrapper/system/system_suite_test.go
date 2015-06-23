package system_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"testing"
)

func TestBootstrapperSystem(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bootstrapper System Suite")
}
