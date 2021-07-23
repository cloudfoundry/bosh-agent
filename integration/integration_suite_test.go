package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	. "github.com/cloudfoundry/bosh-agent/integration"
)

var (
	testEnvironment *TestEnvironment
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		logLevel := boshlog.LevelError
		logger := boshlog.NewLogger(logLevel)
		cmdRunner := boshsys.NewExecCmdRunner(logger)
		var err error
		testEnvironment, err = NewTestEnvironment(cmdRunner, logLevel)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StopAgent()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.EnsureRootDeviceIsLargeEnough()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.DetachLoopDevices()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.ResetDeviceMap()
		Expect(err).ToNot(HaveOccurred())
	})

	RunSpecs(t, "Integration Suite")
}
