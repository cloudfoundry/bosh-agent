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

		err = testEnvironment.StopAgent()
		Expect(err).ToNot(HaveOccurred())
		// create a backup of original settings for nats FW tests
		_, err = testEnvironment.RunCommand("sudo sh -c \"mkdir -p /settings-backup && cp /var/vcap/bosh/*.json /settings-backup/ \" ")
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupDataDir()
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

	AfterEach(func() {

		err := testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupLogFile()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.ResetDeviceMap()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StopAgentTunnel()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.StopAgent()
		Expect(err).NotTo(HaveOccurred())
	})

	RunSpecs(t, "Integration Suite")
}
