package infrastructure_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
)

var _ = Describe("OpenstackMetadataServiceProvider", func() {
	var (
		openstackMetadataServiceProvider MetadataServiceProvider
		fakeresolver                     *fakeinf.FakeDNSResolver
		platform                         *fakeplatform.FakePlatform
		logger                           boshlog.Logger
	)

	BeforeEach(func() {
		fakeresolver = &fakeinf.FakeDNSResolver{}
		platform = fakeplatform.NewFakePlatform()
		logger = boshlog.NewLogger(boshlog.LevelNone)
		openstackMetadataServiceProvider = NewOpenstackMetadataServiceProvider(
			fakeresolver,
			platform,
			MetadataServiceOptions{UseConfigDrive: true},
			logger,
		)
	})

	Describe("GetMetadataService", func() {
		Context("when UseConfigDrive option is set", func() {
			Context("when config drive metadata service is successfully loaded", func() {
				BeforeEach(func() {
					metadataContents, err := json.Marshal(MetadataContentsType{})
					Expect(err).ToNot(HaveOccurred())

					platform.SetGetFilesContentsFromDisk("ec2/latest/meta-data.json", metadataContents, nil)
					platform.SetGetFilesContentsFromDisk("ec2/latest/user-data", []byte("{}"), nil)
				})

				It("returns a multi source metadata service", func() {
					configDriveDiskPaths := []string{
						"/dev/disk/by-label/CONFIG-2",
						"/dev/disk/by-label/config-2",
						"/dev/disk/by-label/METADATA",
					}

					configDriveMetadataService := NewConfigDriveMetadataService(
						fakeresolver,
						platform,
						configDriveDiskPaths,
						"ec2/latest/meta-data.json",
						"ec2/latest/user-data",
						logger,
					)

					httpMetadataService := NewHTTPMetadataService("http://169.254.169.254", fakeresolver)

					metadataService := openstackMetadataServiceProvider.Get()
					multiSourceService, ok := metadataService.(*MultiSourceMetadataService)
					Expect(ok).To(BeTrue())

					Expect(multiSourceService.Services[0]).To(Equal(configDriveMetadataService))
					Expect(multiSourceService.Services[1]).To(Equal(httpMetadataService))
				})
			})
		})

		Context("when UseConfigDrive option is not set", func() {
			BeforeEach(func() {
				openstackMetadataServiceProvider = NewOpenstackMetadataServiceProvider(
					fakeresolver,
					platform,
					MetadataServiceOptions{UseConfigDrive: false},
					logger,
				)
			})

			It("returns http metadata service", func() {
				expectedMetadataService := NewHTTPMetadataService("http://169.254.169.254", fakeresolver)
				Expect(openstackMetadataServiceProvider.Get()).To(Equal(expectedMetadataService))
			})
		})
	})
})
