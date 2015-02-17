package infrastructure_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"

	fakedpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver/fakes"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("genericInfrastructure", func() {
	var (
		inf      Infrastructure
		platform *fakeplatform.FakePlatform

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
			networkingType,
			staticEphemeralDiskPath,
			logger,
		)
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
