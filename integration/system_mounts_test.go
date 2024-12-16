package integration_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("SystemMounts", func() {
	var (
		fileSettings boshsettings.Settings
	)

	Context("mounting /tmp", func() {

		BeforeEach(func() {

			err := testEnvironment.UpdateAgentConfig("file-settings-agent-no-default-tmp-dir.json")
			Expect(err).ToNot(HaveOccurred())

			networks, err := testEnvironment.GetVMNetworks()
			Expect(err).ToNot(HaveOccurred())

			fileSettings = boshsettings.Settings{
				Blobstore: boshsettings.Blobstore{
					Type: "local",
					Options: map[string]interface{}{
						"blobstore_path": "/var/vcap/data",
					},
				},
				Networks: networks,
			}
		})

		Context("when ephemeral disk exists", func() {
			BeforeEach(func() {
				err := testEnvironment.AttachDevice("/dev/sdh", 128, 2)
				Expect(err).ToNot(HaveOccurred())

				fileSettings.Disks = boshsettings.Disks{
					Ephemeral: "/dev/sdh",
				}

				err = testEnvironment.CreateSettingsFile(fileSettings)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				err := testEnvironment.DetachDevice("/dev/sdh")
				Expect(err).ToNot(HaveOccurred())

				_, err = testEnvironment.RunCommand("! mount | grep -q ' on /tmp ' || sudo umount /tmp")
				Expect(err).ToNot(HaveOccurred())

				_, err = testEnvironment.RunCommand("! mount | grep -q ' on /var/tmp ' || sudo umount /var/tmp")
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when agent is first started", func() {
				It("binds /var/vcap/data/root_tmp on /tmp", func() {
					Eventually(func() string {
						result, _ := testEnvironment.RunCommand("sudo findmnt -D /tmp | grep -c '[/root_tmp]'")
						return strings.TrimSpace(result)
					}, 2*time.Minute, 1*time.Second).Should(Equal("1"))

					result, err := testEnvironment.RunCommand("stat -c %a /tmp")
					Expect(err).ToNot(HaveOccurred())
					Expect(strings.TrimSpace(result)).To(Equal("1777"))
				})

				It("binds /var/vcap/data/root_tmp on /var/tmp", func() {
					Eventually(func() string {
						result, _ := testEnvironment.RunCommand("sudo findmnt -D /var/tmp | grep -c '[/root_tmp]'")
						return strings.TrimSpace(result)
					}, 2*time.Minute, 1*time.Second).Should(Equal("1"))

					result, err := testEnvironment.RunCommand("stat -c %a /var/tmp")
					Expect(err).ToNot(HaveOccurred())
					Expect(strings.TrimSpace(result)).To(Equal("1777"))
				})
			})

			Context("when agent is restarted", func() {
				It("does not change mounts and permissions", func() {
					waitForAgentAndExpectMounts := func() {
						Eventually(func() bool {
							return testEnvironment.LogFileContains("sv start monit")
						}, 2*time.Minute, 1*time.Second).Should(BeTrue())

						result, _ := testEnvironment.RunCommand("sudo findmnt -D /tmp | grep -c '[/root_tmp]'")
						Expect(strings.TrimSpace(result)).To(Equal("1"))

						result, _ = testEnvironment.RunCommand("sudo findmnt -D /var/tmp | grep -c '[/root_tmp]'")
						Expect(strings.TrimSpace(result)).To(Equal("1"))

						result, err := testEnvironment.RunCommand("stat -c %a /tmp")
						Expect(err).ToNot(HaveOccurred())
						Expect(strings.TrimSpace(result)).To(Equal("1777"))

						result, err = testEnvironment.RunCommand("stat -c %a /var/tmp")
						Expect(err).ToNot(HaveOccurred())
						Expect(strings.TrimSpace(result)).To(Equal("1777"))
					}

					waitForAgentAndExpectMounts()

					err := testEnvironment.CleanupLogFile()
					Expect(err).ToNot(HaveOccurred())

					err = testEnvironment.RestartAgent()
					Expect(err).ToNot(HaveOccurred())

					waitForAgentAndExpectMounts()
				})
			})

			Context("when the bind-mounts are removed", func() {
				It("has permission 777 on /tmp", func() {
					Eventually(func() string {
						result, _ := testEnvironment.RunCommand("sudo findmnt -D /tmp | grep -c '[/root_tmp]'")
						return strings.TrimSpace(result)
					}, 2*time.Minute, 1*time.Second).Should(Equal("1"))

					_, err := testEnvironment.RunCommand("sudo umount /tmp")
					Expect(err).ToNot(HaveOccurred())

					result, err := testEnvironment.RunCommand("stat -c %a /tmp")
					Expect(err).ToNot(HaveOccurred())
					Expect(strings.TrimSpace(result)).To(Equal("1777"))
				})

				It("has permission 770 on /var/tmp", func() {
					Eventually(func() string {
						result, _ := testEnvironment.RunCommand("sudo findmnt -D /var/tmp | grep -c '[/root_tmp]'")
						return strings.TrimSpace(result)
					}, 2*time.Minute, 1*time.Second).Should(Equal("1"))

					_, err := testEnvironment.RunCommand("sudo umount /var/tmp")
					Expect(err).ToNot(HaveOccurred())

					result, err := testEnvironment.RunCommand("stat -c %a /var/tmp")
					Expect(err).ToNot(HaveOccurred())
					Expect(strings.TrimSpace(result)).To(Equal("1777"))
				})
			})
		})
	})
})
