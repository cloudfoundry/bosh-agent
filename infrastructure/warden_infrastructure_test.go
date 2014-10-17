package infrastructure_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	fakedpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver/fakes"
	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("wardenInfrastructure", func() {
	var (
		platform     *fakeplatform.FakePlatform
		inf          Infrastructure
		fakeRegistry *fakeinf.FakeRegistry
	)

	BeforeEach(func() {
		platform = fakeplatform.NewFakePlatform()
		fakeDevicePathResolver := fakedpresolv.NewFakeDevicePathResolver()
		registryProvider := &fakeinf.FakeRegistryProvider{}
		fakeRegistry = &fakeinf.FakeRegistry{}
		registryProvider.GetRegistryRegistry = fakeRegistry
		inf = NewWardenInfrastructure(platform, fakeDevicePathResolver, registryProvider)
	})

	Describe("GetSettings", func() {
		var expectedSettings boshsettings.Settings

		BeforeEach(func() {
			expectedSettings = boshsettings.Settings{
				AgentID: "fake-agent-id",
			}
			fakeRegistry.Settings = expectedSettings
		})

		It("returns settings", func() {
			settings, err := inf.GetSettings()
			Expect(err).ToNot(HaveOccurred())
			Expect(settings).To(Equal(expectedSettings))
		})
	})
})
