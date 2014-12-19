package infrastructure_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakedpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver/fakes"
	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
)

func init() {
	Describe("openstackInfrastructure", func() {
		var (
			metadataService    *fakeinf.FakeMetadataService
			registry           *fakeinf.FakeRegistry
			platform           *fakeplatform.FakePlatform
			devicePathResolver *fakedpresolv.FakeDevicePathResolver
			openstack          Infrastructure
		)

		BeforeEach(func() {
			metadataService = &fakeinf.FakeMetadataService{}
			registry = &fakeinf.FakeRegistry{}
			platform = fakeplatform.NewFakePlatform()
			devicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
			logger := boshlog.NewLogger(boshlog.LevelNone)
			openstack = NewOpenstackInfrastructure(metadataService, registry, platform, devicePathResolver, logger)
		})

		Describe("NewOpenstackRegistry", func() {
			It("returns concrete registry with useServerNameAsID set to true", func() {
				expectedRegistry := NewHTTPRegistry(metadataService, true)
				Expect(NewOpenstackRegistry(metadataService)).To(Equal(expectedRegistry))
			})
		})

		Describe("SetupSSH", func() {
			It("gets the public key and sets up ssh via the platform", func() {
				metadataService.PublicKey = "fake-public-key"

				err := openstack.SetupSSH("vcap")
				Expect(err).NotTo(HaveOccurred())

				Expect(platform.SetupSSHPublicKey).To(Equal("fake-public-key"))
				Expect(platform.SetupSSHUsername).To(Equal("vcap"))
			})

			It("returns error without configuring ssh on the platform if getting public key fails", func() {
				metadataService.GetPublicKeyErr = errors.New("fake-get-public-key-err")

				err := openstack.SetupSSH("vcap")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-public-key-err"))

				Expect(platform.SetupSSHCalled).To(BeFalse())
			})

			It("returns error if configuring ssh on the platform fails", func() {
				platform.SetupSSHErr = errors.New("fake-setup-ssh-err")

				err := openstack.SetupSSH("vcap")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-setup-ssh-err"))
			})
		})

		Describe("GetSettings", func() {
			It("gets settings", func() {
				settings := boshsettings.Settings{AgentID: "fake-agent-id"}
				registry.Settings = settings

				settings, err := openstack.GetSettings()
				Expect(err).ToNot(HaveOccurred())

				Expect(settings).To(Equal(settings))
			})

			It("returns an error when registry fails to get settings", func() {
				registry.GetSettingsErr = errors.New("fake-get-settings-err")

				settings, err := openstack.GetSettings()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-settings-err"))

				Expect(settings).To(Equal(boshsettings.Settings{}))
			})
		})

		Describe("SetupNetworking", func() {
			It("sets up DHCP on the platform", func() {
				networks := boshsettings.Networks{"bosh": boshsettings.Network{}}

				err := openstack.SetupNetworking(networks)
				Expect(err).ToNot(HaveOccurred())

				Expect(platform.SetupDhcpNetworks).To(Equal(networks))
			})

			It("returns error if configuring DHCP fails", func() {
				platform.SetupDhcpErr = errors.New("fake-setup-dhcp-err")

				err := openstack.SetupNetworking(boshsettings.Networks{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-setup-dhcp-err"))
			})
		})

		Describe("GetEphemeralDiskPath", func() {
			It("returns the real disk path given an openstack hint", func() {
				platform.NormalizeDiskPathRealPath = "/dev/xvdb"
				diskSettings := boshsettings.DiskSettings{Path: "/dev/sdb"}
				realPath := openstack.GetEphemeralDiskPath(diskSettings)
				Expect(realPath).To(Equal("/dev/xvdb"))
				Expect(platform.NormalizeDiskPathSettings).To(Equal(diskSettings))
			})

			It("returns false if path cannot be normalized", func() {
				platform.NormalizeDiskPathRealPath = ""
				diskSettings := boshsettings.DiskSettings{Path: "/dev/sdb"}
				realPath := openstack.GetEphemeralDiskPath(diskSettings)
				Expect(realPath).To(Equal(""))
				Expect(platform.NormalizeDiskPathSettings).To(Equal(diskSettings))
			})

			It("returns an empty string to indicated that there is no ephemeral disk path if device path is empty", func() {
				realPath := openstack.GetEphemeralDiskPath(boshsettings.DiskSettings{})
				Expect(realPath).To(BeEmpty())
				Expect(platform.NormalizeDiskPathCalled).To(BeFalse())
			})
		})
	})
}
