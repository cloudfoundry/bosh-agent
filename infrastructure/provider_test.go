package infrastructure_test

import (
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	// boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	// fakesys "github.com/cloudfoundry/bosh-agent/system/fakes"
)

var _ = Describe("Provider", func() {
	var (
		logger   boshlog.Logger
		platform *fakeplatform.FakePlatform
		provider Provider
	)

	BeforeEach(func() {
		platform = fakeplatform.NewFakePlatform()
		options := ProviderOptions{}
		logger = boshlog.NewLogger(boshlog.LevelNone)
		provider = NewProvider(platform, options, logger)
	})

	Describe("Get", func() {
		It("returns infrastructure", func() {
			// todo infr
			// resolver := NewRegistryEndpointResolver(
			// 	NewDigDNSResolver(logger),
			// )

			// metadataService := NewAwsMetadataServiceProvider(resolver).Get()
			// registry := NewAwsRegistry(metadataService)

			// expectedDevicePathResolver := boshdpresolv.NewMappedDevicePathResolver(
			// 	500*time.Millisecond,
			// 	platform.GetFs(),
			// )

			// expectedInf := NewAwsInfrastructure(
			// 	metadataService,
			// 	registry,
			// 	platform,
			// 	expectedDevicePathResolver,
			// 	logger,
			// )

			// inf, err := provider.Get("aws")
			// Expect(err).ToNot(HaveOccurred())
			// Expect(inf).To(Equal(expectedInf))
		})

		It("returns an error on unknown infrastructure", func() {
			// _, err := provider.Get("some unknown infrastructure name")
			// Expect(err).To(HaveOccurred())
		})
	})
})
