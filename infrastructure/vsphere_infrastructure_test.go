package infrastructure_test

import (
	fakedpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver/fakes"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
)

var _ = Describe("vSphere Infrastructure", func() {
	var (
		logger             boshlog.Logger
		vsphere            Infrastructure
		platform           *fakeplatform.FakePlatform
		devicePathResolver *fakedpresolv.FakeDevicePathResolver
	)

	BeforeEach(func() {
		platform = fakeplatform.NewFakePlatform()
		devicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
		logger = boshlog.NewLogger(boshlog.LevelNone)
	})

	JustBeforeEach(func() {
		vsphere = NewVsphereInfrastructure(platform, devicePathResolver, logger)
	})

	Describe("GetSettings", func() {
		It("vsphere get settings", func() {
			platform.GetFileContentsFromCDROMContents = []byte(`{"agent_id": "123"}`)

			settings, err := vsphere.GetSettings()
			Expect(err).NotTo(HaveOccurred())

			Expect(platform.GetFileContentsFromCDROMPath).To(Equal("env"))
			Expect(settings.AgentID).To(Equal("123"))
		})
	})

	Describe("SetupNetworking", func() {
		It("vsphere setup networking", func() {
			networks := boshsettings.Networks{"bosh": boshsettings.Network{}}

			vsphere.SetupNetworking(networks)

			Expect(platform.SetupManualNetworkingNetworks).To(Equal(networks))
		})
	})

	Describe("GetEphemeralDiskPath", func() {
		It("vsphere get ephemeral disk path", func() {
			realPath := vsphere.GetEphemeralDiskPath(boshsettings.DiskSettings{Path: "does not matter"})
			Expect(realPath).To(Equal("/dev/sdb"))
		})
	})
})
