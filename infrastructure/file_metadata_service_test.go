package infrastructure_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakesys "github.com/cloudfoundry/bosh-agent/system/fakes"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
)

var _ = Describe("FileMetadataService", func() {
	var (
		fs              *fakesys.FakeFileSystem
		metadataService MetadataService
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		metadataService = NewFileMetadataService("fake-userdata-file-path", "fake-metadata-file-path", fs, logger)
	})

	Describe("GetInstanceID", func() {
		Context("when metadata service file exists", func() {
			BeforeEach(func() {
				metadataContents := `{"instance-id":"fake-instance-id"}`
				fs.WriteFileString("fake-metadata-file-path", metadataContents)
			})

			It("returns instance id", func() {
				instanceID, err := metadataService.GetInstanceID()
				Expect(err).NotTo(HaveOccurred())
				Expect(instanceID).To(Equal("fake-instance-id"))
			})
		})

		Context("when metadata service file does not exist", func() {
			It("returns an error", func() {
				_, err := metadataService.GetInstanceID()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetRegistryEndpoint", func() {
		Context("when metadata service file exists", func() {
			BeforeEach(func() {
				userDataContents := `{"registry":{"endpoint":"fake-registry-endpoint"}}`
				fs.WriteFileString("fake-userdata-file-path", userDataContents)
			})

			It("returns registry endpoint", func() {
				registryEndpoint, err := metadataService.GetRegistryEndpoint()
				Expect(err).NotTo(HaveOccurred())
				Expect(registryEndpoint).To(Equal("fake-registry-endpoint"))
			})
		})

		Context("when metadata service file does not exist", func() {
			It("returns an error", func() {
				_, err := metadataService.GetRegistryEndpoint()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("IsAvailable", func() {
		It("returns true", func() {
			Expect(metadataService.IsAvailable()).To(BeTrue())
		})
	})
})
