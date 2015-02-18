package bootstrap_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/bootstrap"
	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
)

func init() {
	Describe("bootstrap", func() {
		Describe("Run", func() {
			var (
				inf         *fakeinf.FakeInfrastructure
				platform    *fakeplatform.FakePlatform
				dirProvider boshdir.Provider

				settingsSource  *fakeinf.FakeSettingsSource
				settingsService *fakesettings.FakeSettingsService
			)

			BeforeEach(func() {
				inf = &fakeinf.FakeInfrastructure{
					GetEphemeralDiskPathRealPath: "/dev/sdz",
				}
				platform = fakeplatform.NewFakePlatform()
				dirProvider = boshdir.NewProvider("/var/vcap")

				settingsSource = &fakeinf.FakeSettingsSource{}
				settingsService = &fakesettings.FakeSettingsService{}
			})

			bootstrap := func() error {
				logger := boshlog.NewLogger(boshlog.LevelNone)
				return New(inf, platform, dirProvider, settingsSource, settingsService, logger).Run()
			}

			It("sets up runtime configuration", func() {
				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(platform.SetupRuntimeConfigurationWasInvoked).To(BeTrue())
			})

			Describe("SSH tunnel setup for registry", func() {
				It("returns error without configuring ssh on the platform if getting public key fails", func() {
					settingsSource.PublicKeyErr = errors.New("fake-get-public-key-err")

					err := bootstrap()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-get-public-key-err"))

					Expect(platform.SetupSSHCalled).To(BeFalse())
				})

				Context("when public key is not empty", func() {
					BeforeEach(func() {
						settingsSource.PublicKey = "fake-public-key"
					})

					It("gets the public key and sets up ssh via the platform", func() {
						err := bootstrap()
						Expect(err).NotTo(HaveOccurred())

						Expect(platform.SetupSSHPublicKey).To(Equal("fake-public-key"))
						Expect(platform.SetupSSHUsername).To(Equal("vcap"))
					})

					It("returns error if configuring ssh on the platform fails", func() {
						platform.SetupSSHErr = errors.New("fake-setup-ssh-err")

						err := bootstrap()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-setup-ssh-err"))
					})
				})

				Context("when public key key is empty", func() {
					BeforeEach(func() {
						settingsSource.PublicKey = ""
					})

					It("gets the public key and does not setup SSH", func() {
						err := bootstrap()
						Expect(err).NotTo(HaveOccurred())

						Expect(platform.SetupSSHCalled).To(BeFalse())
					})
				})
			})

			It("sets up hostname", func() {
				settingsService.Settings.AgentID = "foo-bar-baz-123"

				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(platform.SetupHostnameHostname).To(Equal("foo-bar-baz-123"))
			})

			It("fetches initial settings", func() {
				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(settingsService.SettingsWereLoaded).To(BeTrue())
			})

			It("returns error from loading initial settings", func() {
				settingsService.LoadSettingsError = errors.New("fake-load-error")

				err := bootstrap()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-load-error"))
			})

			It("sets up networking", func() {
				networks := boshsettings.Networks{
					"bosh": boshsettings.Network{},
				}
				settingsService.Settings.Networks = networks

				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(inf.SetupNetworkingNetworks).To(Equal(networks))
			})

			It("sets up ephemeral disk", func() {
				settingsService.Settings.Disks = boshsettings.Disks{
					Ephemeral: "fake-ephemeral-disk-setting",
				}

				inf.GetEphemeralDiskPathRealPath = "/dev/sda"

				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(platform.SetupEphemeralDiskWithPathDevicePath).To(Equal("/dev/sda"))
				Expect(inf.GetEphemeralDiskSettings).To(Equal(boshsettings.DiskSettings{
					VolumeID: "fake-ephemeral-disk-setting",
					Path:     "fake-ephemeral-disk-setting",
				}))
			})

			It("returns error if setting ephemeral disk fails", func() {
				platform.SetupEphemeralDiskWithPathErr = errors.New("fake-setup-ephemeral-disk-err")
				err := bootstrap()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-setup-ephemeral-disk-err"))
			})

			It("sets up data dir", func() {
				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(platform.SetupDataDirCalled).To(BeTrue())
			})

			It("returns error if set up of data dir fails", func() {
				platform.SetupDataDirErr = errors.New("fake-setup-data-dir-err")
				err := bootstrap()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-setup-data-dir-err"))
			})

			It("sets up tmp dir", func() {
				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(platform.SetupTmpDirCalled).To(BeTrue())
			})

			It("returns error if set up of tmp dir fails", func() {
				platform.SetupTmpDirErr = errors.New("fake-setup-tmp-dir-err")
				err := bootstrap()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-setup-tmp-dir-err"))
			})

			It("mounts persistent disk", func() {
				settingsService.Settings.Disks = boshsettings.Disks{
					Persistent: map[string]interface{}{
						"vol-123": map[string]interface{}{
							"volume_id": "2",
							"path":      "/dev/sdb",
						},
					},
				}

				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(platform.MountPersistentDiskSettings).To(Equal(boshsettings.DiskSettings{
					ID:       "vol-123",
					VolumeID: "2",
					Path:     "/dev/sdb",
				}))
				Expect(platform.MountPersistentDiskMountPoint).To(Equal(dirProvider.StoreDir()))
			})

			It("errors if there is more than one persistent disk", func() {
				settingsService.Settings.Disks = boshsettings.Disks{
					Persistent: map[string]interface{}{
						"vol-123": "/dev/sdb",
						"vol-456": "/dev/sdc",
					},
				}

				err := bootstrap()
				Expect(err).To(HaveOccurred())
			})

			It("does not try to mount when no persistent disk", func() {
				settingsService.Settings.Disks = boshsettings.Disks{
					Persistent: map[string]interface{}{},
				}

				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(platform.MountPersistentDiskSettings).To(Equal(boshsettings.DiskSettings{}))
				Expect(platform.MountPersistentDiskMountPoint).To(Equal(""))
			})

			It("sets root and vcap passwords", func() {
				settingsService.Settings.Env.Bosh.Password = "some-encrypted-password"

				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(2).To(Equal(len(platform.UserPasswords)))
				Expect("some-encrypted-password").To(Equal(platform.UserPasswords["root"]))
				Expect("some-encrypted-password").To(Equal(platform.UserPasswords["vcap"]))
			})

			It("does not set password if not provided", func() {
				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(0).To(Equal(len(platform.UserPasswords)))
			})

			It("sets ntp", func() {
				settingsService.Settings.Ntp = []string{
					"0.north-america.pool.ntp.org",
					"1.north-america.pool.ntp.org",
				}

				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(2).To(Equal(len(platform.SetTimeWithNtpServersServers)))
				Expect("0.north-america.pool.ntp.org").To(Equal(platform.SetTimeWithNtpServersServers[0]))
				Expect("1.north-america.pool.ntp.org").To(Equal(platform.SetTimeWithNtpServersServers[1]))
			})

			It("setups up monit user", func() {
				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(platform.SetupMonitUserSetup).To(BeTrue())
			})

			It("starts monit", func() {
				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(platform.StartMonitStarted).To(BeTrue())
			})
		})
	})
}
