package infrastructure_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	fakesys "github.com/cloudfoundry/bosh-agent/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
)

var _ = Describe("RegistryProvider", func() {
	var (
		metadataService  *fakeinf.FakeMetadataService
		fs               *fakesys.FakeFileSystem
		registryProvider RegistryProvider
	)

	BeforeEach(func() {
		metadataService = &fakeinf.FakeMetadataService{}
		fs = fakesys.NewFakeFileSystem()
		registryProvider = NewRegistryProvider(metadataService, "fake-fallback-file-registry-path", fs)
	})

	Describe("GetRegistry", func() {
		Context("when metadata service returns registry http endpoint", func() {
			BeforeEach(func() {
				metadataService.RegistryEndpoint = "http://registry-endpoint"
			})

			It("returns an http registry", func() {
				expectedRegistry := NewHTTPRegistry(metadataService, false)
				Expect(registryProvider.GetRegistry()).To(Equal(expectedRegistry))
			})
		})

		Context("when metadata service returns registry file endpoint", func() {
			BeforeEach(func() {
				metadataService.RegistryEndpoint = "/tmp/registry-endpoint"
			})

			It("returns a file registry", func() {
				expectedRegistry := NewFileRegistry("/tmp/registry-endpoint", fs)
				Expect(registryProvider.GetRegistry()).To(Equal(expectedRegistry))
			})
		})

		Context("when metadata service returns an error", func() {
			BeforeEach(func() {
				metadataService.GetRegistryEndpointErr = errors.New("fake-get-registry-endpoint-error")
			})

			It("returns file registry with fallback path", func() {
				expectedRegistry := NewFileRegistry("fake-fallback-file-registry-path", fs)
				Expect(registryProvider.GetRegistry()).To(Equal(expectedRegistry))
			})
		})
	})
})
