package integration_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"github.com/cloudfoundry/bosh-agent/v2/integration"
)

var (
	testEnvironment *integration.TestEnvironment
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		logLevel := boshlog.LevelError
		logger := boshlog.NewLogger(logLevel)
		cmdRunner := boshsys.NewExecCmdRunner(logger)
		var err error
		testEnvironment, err = integration.NewTestEnvironment(cmdRunner, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.DetectServiceManager()
		Expect(err).ToNot(HaveOccurred())

		// Establish the clean baseline every spec's BeforeEach relies on; the AfterEach re-establishes
		// it after each spec via the same helper (see RestoreCleanBaseline).
		err = testEnvironment.RestoreCleanBaseline()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.EnsureRootDeviceIsLargeEnough()
		Expect(err).ToNot(HaveOccurred())

		output, err := testEnvironment.RunCommand("sudo chmod +x /var/vcap/bosh/bin/bosh-agent && sudo /var/vcap/bosh/bin/bosh-agent -v")
		Expect(err).ToNot(HaveOccurred())

		Expect(output).To(ContainSubstring("[DEV BUILD]"))

		return []byte("done")

	}, func(in []byte) {})

	JustBeforeEach(func() {
		err := testEnvironment.StartAgent()
		Expect(err).ToNot(HaveOccurred())
	})

	JustAfterEach(func() {
		if CurrentSpecReport().Failed() {
			testEnvironment.DumpDiagnostics()
		}
	})

	AfterEach(func() {
		// Single teardown path back to the clean baseline (see RestoreCleanBaseline).
		err := testEnvironment.RestoreCleanBaseline()
		Expect(err).ToNot(HaveOccurred())
	})

	RunSpecs(t, "Integration Suite")
}
