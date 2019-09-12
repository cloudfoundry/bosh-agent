package integration_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("ConfigDrive", func() {
	Context("when vm is using config drive", func() {
		BeforeEach(func() {
			err := testEnvironment.StopAgent()
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() error {
				return testEnvironment.SetupConfigDrive()
			}, 1*time.Minute).ShouldNot(HaveOccurred())

			registrySettings := boshsettings.Settings{
				AgentID: "fake-agent-id",
			}

			err = testEnvironment.StartRegistry(registrySettings)
			Expect(err).ToNot(HaveOccurred())

			err = testEnvironment.UpdateAgentConfig("config-drive-agent.json")
			Expect(err).ToNot(HaveOccurred())

			err = testEnvironment.StartAgent()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := testEnvironment.CleanupDataDir()
			Expect(err).ToNot(HaveOccurred())

			err = testEnvironment.DetachLoopDevices()
			Expect(err).ToNot(HaveOccurred())
		})

		It("using config drive to get registry URL", func() {
			var settingsJSON string
			var err error
			Eventually(func() error {
				settingsJSON, err = testEnvironment.GetFileContents("/var/vcap/bosh/settings.json")
				return err
			}).ShouldNot(HaveOccurred())
			Expect(settingsJSON).To(ContainSubstring("fake-agent-id"))
		})

		It("config drive is being unmounted", func() {
			Eventually(func() int {
				result, _ := testEnvironment.RunCommand("sudo mount")
				return strings.Count(result, "/dev/loop2")
			}, 5*time.Second, 1*time.Second).Should(BeZero())
		})
	})
})
