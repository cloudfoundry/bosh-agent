package integration_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsys "github.com/cloudfoundry/bosh-agent/system"

	. "github.com/cloudfoundry/bosh-agent/integration"
)

var (
	testEnvironment *TestEnvironment
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		logger := boshlog.NewLogger(boshlog.LevelDebug)
		cmdRunner := boshsys.NewExecCmdRunner(logger)
		testEnvironment = NewTestEnvironment(cmdRunner)
	})

	RunSpecs(t, "Integration Suite")
}
