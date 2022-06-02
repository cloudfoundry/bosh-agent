package infrastructure_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	"github.com/cloudfoundry/bosh-utils/errors"
)

var _ = Describe("MultiSourceMetadataService", describeMultiSourceMetadataService)

func describeMultiSourceMetadataService() { //nolint:funlen
	var (
		metadataService MetadataService
		service1        fakeinf.FakeMetadataService
		service2        fakeinf.FakeMetadataService
	)

	BeforeEach(func() {
		service1 = fakeinf.FakeMetadataService{
			Available:  false,
			PublicKey:  "fake-public-key-1",
			InstanceID: "fake-instance-id-1",
			ServerName: "fake-server-name-1",
			Networks:   boshsettings.Networks{"net-1": boshsettings.Network{}},
			Settings: boshsettings.Settings{
				AgentID: "Agent-Foo",
				Mbus:    "Agent-Mbus",
			},
		}

		service2 = fakeinf.FakeMetadataService{
			Available:  false,
			PublicKey:  "fake-public-key-2",
			InstanceID: "fake-instance-id-2",
			ServerName: "fake-server-name-2",
			Networks:   boshsettings.Networks{"net-2": boshsettings.Network{}},
		}
	})

	Context("when the first service is available", func() {
		BeforeEach(func() {
			service1.Available = true
			metadataService = NewMultiSourceMetadataService(service1, service2)
		})

		Describe("IsAvailable", func() {
			It("is true", func() {
				availability := metadataService.IsAvailable()
				Expect(availability).To(BeTrue())
			})
		})

		Describe("GetPublicKey", func() {
			It("returns public key from the available service", func() {
				publicKey, err := metadataService.GetPublicKey()
				Expect(err).NotTo(HaveOccurred())
				Expect(publicKey).To(Equal("fake-public-key-1"))
			})
		})

		Describe("GetInstanceID", func() {
			It("returns instance ID from the available service", func() {
				instanceID, err := metadataService.GetInstanceID()
				Expect(err).NotTo(HaveOccurred())
				Expect(instanceID).To(Equal("fake-instance-id-1"))
			})
		})

		Describe("GetServerName", func() {
			It("returns server name from the available service", func() {
				serverName, err := metadataService.GetServerName()
				Expect(err).NotTo(HaveOccurred())
				Expect(serverName).To(Equal("fake-server-name-1"))
			})
		})

		Describe("GetSettings", func() {
			Context("selected metadata service did not return an error", func() {
				It("returns settings", func() {
					settings, err := metadataService.GetSettings()
					Expect(err).NotTo(HaveOccurred())
					Expect(settings.AgentID).To(Equal("Agent-Foo"))
				})
			})

			Context("selected metadata service returned an error", func() {
				BeforeEach(func() {
					service1.SettingsErr = errors.Error("Foo Bar")
					metadataService = NewMultiSourceMetadataService(service1, service2)
				})

				It("returns the error", func() {
					_, err := metadataService.GetSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Foo Bar"))
				})
			})
		})

		Describe("GetNetworks", func() {
			It("returns network settings from the available service", func() {
				networks, err := metadataService.GetNetworks()
				Expect(err).NotTo(HaveOccurred())
				Expect(networks).To(Equal(boshsettings.Networks{"net-1": boshsettings.Network{}}))
			})
		})
	})

	Context("when the first service is unavailable", func() {
		BeforeEach(func() {
			service1.Available = false
			service2.Available = true
			metadataService = NewMultiSourceMetadataService(service1, service2)
		})

		Describe("IsAvailable", func() {
			It("is true", func() {
				Expect(metadataService.IsAvailable()).To(BeTrue())
			})
		})

		Describe("GetPublicKey", func() {
			It("returns public key from the available service", func() {
				publicKey, err := metadataService.GetPublicKey()
				Expect(err).NotTo(HaveOccurred())
				Expect(publicKey).To(Equal("fake-public-key-2"))
			})
		})

		Describe("GetInstanceID", func() {
			It("returns instance ID from the available service", func() {
				instanceID, err := metadataService.GetInstanceID()
				Expect(err).NotTo(HaveOccurred())
				Expect(instanceID).To(Equal("fake-instance-id-2"))
			})
		})

		Describe("GetServerName", func() {
			It("returns server name from the available service", func() {
				serverName, err := metadataService.GetServerName()
				Expect(err).NotTo(HaveOccurred())
				Expect(serverName).To(Equal("fake-server-name-2"))
			})
		})

		Describe("GetNetworks", func() {
			It("returns network settings from the available service", func() {
				networks, err := metadataService.GetNetworks()
				Expect(err).NotTo(HaveOccurred())
				Expect(networks).To(Equal(boshsettings.Networks{"net-2": boshsettings.Network{}}))
			})
		})
	})

	Context("when no service is available", func() {
		BeforeEach(func() {
			service1.Available = false
			metadataService = NewMultiSourceMetadataService(service1)
		})

		Describe("IsAvailable", func() {
			It("is false", func() {
				Expect(metadataService.IsAvailable()).To(BeFalse())
			})
		})

		Describe("GetPublicKey", func() {
			It("returns an error", func() {
				_, err := metadataService.GetPublicKey()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("services not available"))
			})
		})

		Describe("GetInstanceID", func() {
			It("returns an error getting the instance ID", func() {
				_, err := metadataService.GetInstanceID()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("services not available"))
			})
		})

		Describe("GetServerName", func() {
			It("returns an error getting the server name", func() {
				_, err := metadataService.GetServerName()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("services not available"))
			})
		})

		Describe("GetNetworks", func() {
			It("returns an error getting the networks", func() {
				_, err := metadataService.GetNetworks()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("services not available"))
			})
		})

		Describe("GetSettings", func() {
			It("returns an error getting the settings", func() {
				_, err := metadataService.GetSettings()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("services not available"))
			})
		})
	})
}
