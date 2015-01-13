package integration_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("EphemeralDisk", func() {
	var (
		registrySettings boshsettings.Settings
	)

	BeforeEach(func() {
		err := testEnvironment.StopAgent()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupLogFile()
		Expect(err).ToNot(HaveOccurred())

		// Using openstack to set registry URL in config-drive
		// instead of http metadata service
		err = testEnvironment.SetInfrastructure("openstack")
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.SetupConfigDrive()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.UpdateAgentConfig("config-drive-agent.json")
		Expect(err).ToNot(HaveOccurred())

		registrySettings = boshsettings.Settings{
			AgentID: "fake-agent-id",
			Mbus:    "https://127.0.0.1:6868",
			Blobstore: boshsettings.Blobstore{
				Type: "local",
				Options: map[string]interface{}{
					"blobstore_path": "/var/vcap/data",
				},
			},
		}
	})

	JustBeforeEach(func() {
		err := testEnvironment.StartRegistry(registrySettings)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StartAgent()
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when ephemeral disk is provided in settings", func() {
		BeforeEach(func() {
			registrySettings.Disks = boshsettings.Disks{
				Ephemeral: "/dev/sdh",
			}
		})

		Context("when ephemeral disk exists", func() {
			BeforeEach(func() {
				err := testEnvironment.AttachDevice("/dev/sdh", 128, 2)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				err := testEnvironment.DetachDevice("/dev/sdh")
				Expect(err).ToNot(HaveOccurred())
			})

			It("agent is running", func() {
				Eventually(func() error {
					_, err := testEnvironment.RunCommand("netcat -z -v 127.0.0.1 6868")
					return err
				}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
			})

			It("it is being mounted", func() {
				Eventually(func() string {
					result, _ := testEnvironment.RunCommand("sudo mount | grep /dev/sdh | grep -c /var/vcap/data")
					return strings.TrimSpace(result)
				}, 2*time.Minute, 1*time.Second).Should(Equal("1"))
			})
		})

		Context("when ephemeral disk does not exist", func() {
			BeforeEach(func() {
				testEnvironment.DetachDevice("/dev/sdh")
			})

			It("agent fails with error", func() {
				Eventually(func() bool {
					return testEnvironment.LogFileContains("ERROR .* App setup .* No ephemeral disk found")
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())
			})
		})
	})

	Context("when ephemeral disk is not provided in settings", func() {
		Context("when root disk can not be used as ephemeral", func() {
			It("agent fails with error", func() {
				Eventually(func() bool {
					return testEnvironment.LogFileContains("ERROR .* App setup .* No ephemeral disk found")
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())
			})
		})
	})
})
