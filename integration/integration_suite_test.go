package integration_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	. "github.com/cloudfoundry/bosh-agent/integration"
)

var (
	testEnvironment *TestEnvironment
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		logLevel := boshlog.LevelError
		logger := boshlog.NewLogger(logLevel)
		cmdRunner := boshsys.NewExecCmdRunner(logger)
		var err error
		testEnvironment, err = NewTestEnvironment(cmdRunner, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StopAgent()
		Expect(err).ToNot(HaveOccurred())
		//create a backup of original settings for nats FW tests
		_, err = testEnvironment.RunCommand("sudo sh -c \"mkdir -p /settings-backup && cp /var/vcap/bosh/*.json /settings-backup/ \" ")
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.EnsureRootDeviceIsLargeEnough()
		Expect(err).ToNot(HaveOccurred())

		return []byte("done")

	}, func(in []byte) {})
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
