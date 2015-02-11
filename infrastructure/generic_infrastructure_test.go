package infrastructure_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"

	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	fakedpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver/fakes"
	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("genericInfrastructure", func() {
	var (
		inf            Infrastructure
		platform       *fakeplatform.FakePlatform
		settingsSource *fakeinf.FakeSettingsSource

		firstDevicePathResolver   *fakedpresolv.FakeDevicePathResolver
		secondDevicePathResolver  *fakedpresolv.FakeDevicePathResolver
		defaultDevicePathResolver *fakedpresolv.FakeDevicePathResolver

		devicePathResolutionType string
		networkingType           string
		staticEphemeralDiskPath  string

		logger boshlog.Logger
	)

	BeforeEach(func() {
		platform = fakeplatform.NewFakePlatform()
		settingsSource = &fakeinf.FakeSettingsSource{}
		firstDevicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
		secondDevicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
		defaultDevicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
		devicePathResolutionType = ""
		networkingType = ""
		staticEphemeralDiskPath = ""
		logger = boshlog.NewLogger(boshlog.LevelNone)
	})

	JustBeforeEach(func() {
		inf = NewGenericInfrastructure(
			platform,
			settingsSource,
			map[string]boshdpresolv.DevicePathResolver{
				"fake-dpr1": firstDevicePathResolver,
				"fake-dpr2": secondDevicePathResolver,
			},
			defaultDevicePathResolver,
			devicePathResolutionType,
			networkingType,
			staticEphemeralDiskPath,
			logger,
		)
	})

	Describe("GetDevicePathResolver", func() {
		Context("when infrastructure is configured with known device path resolver", func() {
			BeforeEach(func() { devicePathResolutionType = "fake-dpr2" })

			It("returns matching device path resolver", func() {
				Expect(inf.GetDevicePathResolver()).To(Equal(secondDevicePathResolver))
			})
		})

		Context("when infrastructure is configured with unknown device path resolved", func() {
			It("returns default device path resolver", func() {
				Expect(inf.GetDevicePathResolver()).To(Equal(defaultDevicePathResolver))
			})
		})
	})

	Describe("SetupSSH", func() {
		It("returns error without configuring ssh on the platform if getting public key fails", func() {
			settingsSource.PublicKeyErr = errors.New("fake-get-public-key-err")

			err := inf.SetupSSH("vcap")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-get-public-key-err"))

			Expect(platform.SetupSSHCalled).To(BeFalse())
		})

		Context("when public key is not empty", func() {
			BeforeEach(func() {
				settingsSource.PublicKey = "fake-public-key"
			})

			It("gets the public key and sets up ssh via the platform", func() {
				err := inf.SetupSSH("vcap")
				Expect(err).NotTo(HaveOccurred())

				Expect(platform.SetupSSHPublicKey).To(Equal("fake-public-key"))
				Expect(platform.SetupSSHUsername).To(Equal("vcap"))
			})

			It("returns error if configuring ssh on the platform fails", func() {
				platform.SetupSSHErr = errors.New("fake-setup-ssh-err")

				err := inf.SetupSSH("vcap")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-setup-ssh-err"))
			})
		})

		Context("when public key key is empty", func() {
			BeforeEach(func() {
				settingsSource.PublicKey = ""
			})

			It("gets the public key and does not setup SSH", func() {
				err := inf.SetupSSH("vcap")
				Expect(err).NotTo(HaveOccurred())

				Expect(platform.SetupSSHCalled).To(BeFalse())
			})
		})
	})

	Describe("GetSettings", func() {
		It("returns settings from settings source", func() {
			settings := boshsettings.Settings{AgentID: "fake-agent-id"}
			settingsSource.SettingsValue = settings
			Expect(inf.GetSettings()).To(Equal(settings))
		})
	})

	Describe("SetupNetworking", func() {
		networks := boshsettings.Networks{"bosh": boshsettings.Network{}}

		Context("when infrastructure is configured with 'dhcp'", func() {
			BeforeEach(func() { networkingType = "dhcp" })

			It("sets up DHCP networking on the platform", func() {
				err := inf.SetupNetworking(networks)
				Expect(err).ToNot(HaveOccurred())

				Expect(platform.SetupDhcpNetworks).To(Equal(networks))
			})

			It("returns error if configuring DHCP fails", func() {
				platform.SetupDhcpErr = errors.New("fake-setup-dhcp-err")

				err := inf.SetupNetworking(networks)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-setup-dhcp-err"))
			})

			It("does not set up manual networking on the platform", func() {
				err := inf.SetupNetworking(networks)
				Expect(err).ToNot(HaveOccurred())
				Expect(platform.SetupManualNetworkingCalled).To(BeFalse())
			})
		})

		Context("when infrastructure is configured with 'manual'", func() {
			BeforeEach(func() { networkingType = "manual" })

			It("sets up manual networking on the platform", func() {
				err := inf.SetupNetworking(networks)
				Expect(err).ToNot(HaveOccurred())
				Expect(platform.SetupManualNetworkingNetworks).To(Equal(networks))
			})

			It("returns error if configuring manual networking fails", func() {
				platform.SetupManualNetworkingErr = errors.New("fake-setup-manual-err")

				err := inf.SetupNetworking(networks)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-setup-manual-err"))
			})

			It("does not set up DHCP networking on the platform", func() {
				err := inf.SetupNetworking(networks)
				Expect(err).ToNot(HaveOccurred())
				Expect(platform.SetupDhcpCalled).To(BeFalse())
			})
		})

		Context("when infrastructure is not configured", func() {
			It("does not set up DHCP networking on the platform", func() {
				err := inf.SetupNetworking(networks)
				Expect(err).ToNot(HaveOccurred())
				Expect(platform.SetupDhcpCalled).To(BeFalse())
			})

			It("does not set up manual networking on the platform", func() {
				err := inf.SetupNetworking(networks)
				Expect(err).ToNot(HaveOccurred())
				Expect(platform.SetupManualNetworkingCalled).To(BeFalse())
			})
		})
	})

	Describe("GetEphemeralDiskPath", func() {
		Context("when infrastructure is configured with static ephemeral disk path", func() {
			BeforeEach(func() { staticEphemeralDiskPath = "/dev/sdb" })

			Context("when device path is an empty string", func() {
				It("returns an empty string", func() {
					diskSettings := boshsettings.DiskSettings{
						ID:       "fake-id",
						VolumeID: "fake-volume-id",
						Path:     "",
					}
					Expect(inf.GetEphemeralDiskPath(diskSettings)).To(BeEmpty())
				})
			})

			Context("when device path is not empty string", func() {
				It("returns static disk path", func() {
					diskSettings := boshsettings.DiskSettings{
						ID:       "fake-id",
						VolumeID: "fake-volume-id",
						Path:     "fake-path",
					}
					Expect(inf.GetEphemeralDiskPath(diskSettings)).To(Equal("/dev/sdb"))
				})
			})
		})

		Context("when infrastructure is not configured with static disk path", func() {
			BeforeEach(func() { platform.NormalizeDiskPathRealPath = "/dev/xvdb" })

			Context("when device path is an empty string", func() {
				It("returns an empty string", func() {
					diskSettings := boshsettings.DiskSettings{
						ID:       "fake-id",
						VolumeID: "fake-volume-id",
						Path:     "",
					}
					Expect(inf.GetEphemeralDiskPath(diskSettings)).To(BeEmpty())
				})
			})

			Context("when device path is not empty string", func() {
				It("returns normalized disk path", func() {
					diskSettings := boshsettings.DiskSettings{
						ID:       "fake-id",
						VolumeID: "fake-volume-id",
						Path:     "/dev/sdb",
					}

					Expect(inf.GetEphemeralDiskPath(diskSettings)).To(Equal("/dev/xvdb"))
					Expect(platform.NormalizeDiskPathSettings).To(Equal(diskSettings))
				})
			})
		})
	})
})
