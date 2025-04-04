package integration_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("EphemeralDisk", func() {
	var (
		fileSettings settings.Settings
	)

	Context("mounted on /var/vcap/data", func() {
		BeforeEach(func() {
			err := testEnvironment.CleanupDataDir()
			Expect(err).ToNot(HaveOccurred())

			err = testEnvironment.CleanupLogFile()
			Expect(err).ToNot(HaveOccurred())

			err = testEnvironment.UpdateAgentConfig("file-settings-agent.json")
			Expect(err).ToNot(HaveOccurred())

			networks, err := testEnvironment.GetVMNetworks()
			Expect(err).ToNot(HaveOccurred())

			fileSettings = settings.Settings{
				Blobstore: settings.Blobstore{
					Type: "local",
					Options: map[string]interface{}{
						"blobstore_path": "/var/vcap/data",
					},
				},
				Networks: networks,
			}

		})

		Context("when ephemeral disk is provided in settings", func() {

			BeforeEach(func() {

				fileSettings.Disks = settings.Disks{
					Ephemeral: "/dev/sdh",
				}
				err := testEnvironment.CreateSettingsFile(fileSettings)
				Expect(err).ToNot(HaveOccurred())

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
						result, _ := testEnvironment.RunCommand("sudo mount | grep /dev/sdh | grep -c /var/vcap/data") //nolint:errcheck
						return strings.TrimSpace(result)
					}, 2*time.Minute, 1*time.Second).Should(Equal("1"))
				})

				Context("when bind mount /var/vcap/data/root_tmp on /tmp", func() {
					BeforeEach(func() {
						err := testEnvironment.UpdateAgentConfig("bind-mount-agent.json")
						Expect(err).ToNot(HaveOccurred())
					})

					JustBeforeEach(func() {
						Eventually(func() string {
							result, _ := testEnvironment.RunCommand("{ sudo findmnt /var/tmp; sudo findmnt /tmp; } | grep -c '/dev/sdh2\\[/root_tmp\\]'") //nolint:errcheck
							return strings.TrimSpace(result)
						}, 2*time.Minute, 1*time.Second).Should(Equal("2"))
					})

					It("does not execute executables", func() {
						_, err := testEnvironment.RunCommand("cp /bin/echo /tmp/echo")
						Expect(err).ToNot(HaveOccurred())

						_, err = testEnvironment.RunCommand("/tmp/echo hello")
						Expect(err).To(HaveOccurred())
					})

					It("does not allow device files", func() {
						_, err := testEnvironment.RunCommand("sudo mknod /tmp/blockDevice b 7 98")
						Expect(err).ToNot(HaveOccurred())

						_, err = testEnvironment.RunCommand("sudo dd if=/tmp/blockDevice bs=1M count=10")
						Expect(err).To(HaveOccurred())
					})
				})
			})

			Context("when ephemeral disk does not exist", func() {
				BeforeEach(func() {
					err := testEnvironment.DetachDevice("/dev/sdh")
					Expect(err).ToNot(HaveOccurred())
				})
				It("agent fails with error", func() {
					Eventually(func() bool {
						return testEnvironment.LogFileContains("ERROR .* App setup .* No ephemeral disk found")
					}, 2*time.Minute, 1*time.Second).Should(BeTrue())
				})
			})
		})

		Context("when ephemeral disk is not provided in settings", func() {
			Context("when root disk can be used as ephemeral", func() {
				var (
					oldRootDevice string
				)

				BeforeEach(func() {
					err := testEnvironment.UpdateAgentConfig("root-partition-agent.json")
					Expect(err).ToNot(HaveOccurred())

					oldRootDevice, err = testEnvironment.AttachPartitionedRootDevice("/dev/sdz", 1224, 128)
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func() {
					err := testEnvironment.DetachPartitionedRootDevice(oldRootDevice, "/dev/sdz")
					Expect(err).ToNot(HaveOccurred())
				})

				It("partitions root disk", func() {
					Eventually(func() string {
						ephemeralDataDevice, err := testEnvironment.RunCommand(`sudo mount | grep "on /var/vcap/data " | cut -d' ' -f1`)
						Expect(err).ToNot(HaveOccurred())

						return strings.TrimSpace(ephemeralDataDevice)
					}, 2*time.Minute, 1*time.Second).Should(Equal("/dev/sdz3"))

					partitionTable, err := testEnvironment.RunCommand("sudo sfdisk -d /dev/sdz")
					Expect(err).ToNot(HaveOccurred())

					Expect(partitionTable).To(MatchRegexp(`/dev/sdz1 : start=\s+1, size=\s+262144, type=83`))
					Expect(partitionTable).To(MatchRegexp(`/dev/sdz2 : start=\s+264192, size=\s+\d+, type=83`))
					Expect(partitionTable).To(MatchRegexp(`/dev/sdz3 : start=\s+\d+, size=\s+\d+, type=83`))
				})

				Context("when swap size is set to 0", func() {
					BeforeEach(func() {
						swapSize := uint64(0)
						fileSettings.Env = settings.Env{
							Bosh: settings.BoshEnv{
								SwapSizeInMB: &swapSize,
							},
						}
						err := testEnvironment.CreateSettingsFile(fileSettings)
						Expect(err).ToNot(HaveOccurred())
					})

					It("does not partition a swap device", func() {
						Eventually(func() string {
							ephemeralDataDevice, err := testEnvironment.RunCommand(`sudo mount | grep "on /var/vcap/data " | cut -d' ' -f1`)
							Expect(err).ToNot(HaveOccurred())

							return strings.TrimSpace(ephemeralDataDevice)
						}, 2*time.Minute, 1*time.Second).Should(Equal("/dev/sdz2"))

						partitionTable, err := testEnvironment.RunCommand("sudo sfdisk -d /dev/sdz")
						Expect(err).ToNot(HaveOccurred())

						Expect(partitionTable).To(ContainSubstring("/dev/sdz1 : start=           1, size=      262144, type=83"))
						Expect(partitionTable).To(ContainSubstring("/dev/sdz2 : start=      264192, size=     2242560, type=83"))
					})
				})
			})

			Context("when root disk can not be used as ephemeral", func() {
				It("agent fails with error", func() {
					Eventually(func() bool {
						return testEnvironment.LogFileContains("ERROR .* App setup .* No ephemeral disk found")
					}, 2*time.Minute, 1*time.Second).Should(BeTrue())
				})
			})
		})
	})
})
