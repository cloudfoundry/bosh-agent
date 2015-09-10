package integration_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	//"time"
)

var _ = Describe("RawEphemeralDisk", func() {
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

		err = testEnvironment.SetupConfigDrive()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.UpdateAgentConfig("config-drive-agent.json")
		Expect(err).ToNot(HaveOccurred())

		networks, err := testEnvironment.GetVMNetworks()
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
			Networks: networks,
		}
	})

	Context("when raw ephemeral disk is provided in settings", func() {
		BeforeEach(func() {
//			testEnvironment.DetachLoopDevice("/dev/loop6")
			err := testEnvironment.AttachDevice("/dev/xvdb", 128, 1)
			Expect(err).ToNot(HaveOccurred())

//			time.Sleep(1 * time.Second)

			err = testEnvironment.AttachDevice("/dev/xvdc", 128, 1)
			Expect(err).ToNot(HaveOccurred())

			registrySettings.Disks = boshsettings.Disks{
				Ephemeral: "/dev/sdh",
				RawEphemeralStorage: true,
				RawEphemeralPaths: []string{"/dev/xvdb", "/dev/xvdc"},
			}

			err = testEnvironment.StartRegistry(registrySettings)
			Expect(err).ToNot(HaveOccurred())

			err = testEnvironment.StartAgent()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			testEnvironment.DetachDevice("/dev/xvdb")
			testEnvironment.DetachDevice("/dev/xvdc")
		})

		It("labels the raw ephemeral paths for unpartitioned disks", func(){
			output, err := testEnvironment.RunCommand("find /dev/disk/by-partlabel")

			Expect(err).ToNot(HaveOccurred())
			Expect(output).To(ContainSubstring("/dev/disk/by-partlabel/raw-ephemeral-0"))
			Expect(output).To(ContainSubstring("/dev/disk/by-partlabel/raw-ephemeral-1"))
		})
	})


})
