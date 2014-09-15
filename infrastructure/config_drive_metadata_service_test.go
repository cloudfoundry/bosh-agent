package infrastructure_test

import (
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
)

var _ = Describe("ConfigDriveMetadataService", func() {
	var (
		configDriveMetadataService MetadataService
		resolver                   *fakeinf.FakeDNSResolver
		platform                   *fakeplatform.FakePlatform
	)

	updateMetadata := func(metadataContents MetadataContentsType) {
		metadataJSON, err := json.Marshal(metadataContents)
		Expect(err).ToNot(HaveOccurred())
		platform.SetGetFileContentsFromDisk("ec2/latest/meta-data.json", metadataJSON, nil)

		err = configDriveMetadataService.Load()
		Expect(err).ToNot(HaveOccurred())
	}

	updateUserdata := func(userdataContents string) {
		platform.SetGetFileContentsFromDisk("ec2/latest/user-data", []byte(userdataContents), nil)

		err := configDriveMetadataService.Load()
		Expect(err).ToNot(HaveOccurred())
	}

	BeforeEach(func() {
		resolver = &fakeinf.FakeDNSResolver{}
		platform = fakeplatform.NewFakePlatform()
		diskPaths := []string{
			"fake-disk-path-1",
			"fake-disk-path-2",
		}
		configDriveMetadataService = NewConfigDriveMetadataService(resolver, platform, diskPaths)

		userdataContents := fmt.Sprintf(`{"server":{"name":"fake-server-name"},"registry":{"endpoint":"fake-registry-endpoint"}}`)
		platform.SetGetFileContentsFromDisk("ec2/latest/user-data", []byte(userdataContents), nil)

		metadata := MetadataContentsType{
			PublicKeys: map[string]PublicKeyType{
				"0": PublicKeyType{
					"openssh-key": "fake-openssh-key",
				},
			},
			InstanceID: "fake-instance-id",
		}
		updateMetadata(metadata)
	})

	Describe("Load", func() {
		It("returns an error if it fails to read meta-data.json from disk", func() {
			platform.SetGetFileContentsFromDisk("ec2/latest/meta-data.json", []byte{}, errors.New("fake-read-disk-error"))
			err := configDriveMetadataService.Load()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-read-disk-error"))
		})

		It("tries to load meta-data.json from potential disk locations", func() {
			platform.SetGetFileContentsFromDisk("ec2/latest/meta-data.json", []byte{}, errors.New("fake-read-disk-error"))
			err := configDriveMetadataService.Load()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-read-disk-error"))

			Expect(platform.GetFileContentsFromDiskDiskPaths).To(ContainElement("fake-disk-path-1"))
			Expect(platform.GetFileContentsFromDiskDiskPaths).To(ContainElement("fake-disk-path-2"))
		})

		It("returns an error if it fails to parse meta-data.json contents", func() {
			platform.SetGetFileContentsFromDisk("ec2/latest/meta-data.json", []byte("broken"), nil)
			err := configDriveMetadataService.Load()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Parsing config drive metadata from meta_data.json"))
		})

		It("returns an error if it fails to read user_data from disk", func() {
			platform.SetGetFileContentsFromDisk("ec2/latest/user-data", []byte{}, errors.New("fake-read-disk-error"))
			err := configDriveMetadataService.Load()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-read-disk-error"))
		})

		It("returns an error if it fails to parse user_data contents", func() {
			platform.SetGetFileContentsFromDisk("ec2/latest/user-data", []byte("broken"), nil)
			err := configDriveMetadataService.Load()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Parsing config drive metadata from user_data"))
		})
	})

	Describe("GetPublicKey", func() {
		It("returns public key", func() {
			value, err := configDriveMetadataService.GetPublicKey()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("fake-openssh-key"))
		})

		It("returns an error if it fails to get ssh key", func() {
			updateMetadata(MetadataContentsType{})

			value, err := configDriveMetadataService.GetPublicKey()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load openssh-key from config drive metadata service"))

			Expect(value).To(Equal(""))
		})
	})

	Describe("GetInstanceID", func() {
		It("returns instance id", func() {
			value, err := configDriveMetadataService.GetInstanceID()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("fake-instance-id"))
		})

		It("returns an error if it fails to get instance id", func() {
			updateMetadata(MetadataContentsType{})

			value, err := configDriveMetadataService.GetInstanceID()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load instance-id from config drive metadata service"))

			Expect(value).To(Equal(""))
		})
	})

	Describe("GetServerName", func() {
		It("returns server name", func() {
			value, err := configDriveMetadataService.GetServerName()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("fake-server-name"))
		})

		It("returns an error if it fails to get server name", func() {
			updateUserdata("{}")

			value, err := configDriveMetadataService.GetServerName()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load server name from config drive metadata service"))

			Expect(value).To(Equal(""))
		})
	})

	Describe("GetRegistryEndpoint", func() {
		It("returns registry endpoint", func() {
			value, err := configDriveMetadataService.GetRegistryEndpoint()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("fake-registry-endpoint"))
		})

		It("returns an error if it fails to get registry endpoint", func() {
			updateUserdata("{}")

			value, err := configDriveMetadataService.GetRegistryEndpoint()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load registry endpoint from config drive metadata service"))

			Expect(value).To(Equal(""))
		})
	})
})
