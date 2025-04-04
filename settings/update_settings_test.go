package settings_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("UpdateSettings", func() {
	Describe("MergeSettings", func() {
		var existingSettings UpdateSettings

		BeforeEach(func() {
			existingSettings = UpdateSettings{}
		})

		It("updates trusted certs and disk associations to the new values", func() {
			restartNeeded := existingSettings.MergeSettings(UpdateSettings{
				TrustedCerts: "new certs",
				DiskAssociations: []DiskAssociation{
					{Name: "new disk"},
				},
			})
			Expect(restartNeeded).To(BeFalse())
			Expect(existingSettings.TrustedCerts).To(Equal("new certs"))
			Expect(existingSettings.DiskAssociations[0].Name).To(Equal("new disk"))
		})

		Context("when the existing update settings json contains nats settings", func() {
			BeforeEach(func() {
				existingSettings = UpdateSettings{
					Mbus: MBus{
						Cert: CertKeyPair{
							CA: "existing CA",
						},
					},
				}
			})

			It("does not replace the existing settings with empty values", func() {
				restartNeeded := existingSettings.MergeSettings(UpdateSettings{})
				Expect(restartNeeded).To(BeFalse())
				Expect(existingSettings.Mbus.Cert.CA).To(Equal("existing CA"))
			})

			It("updates nats settings with new values", func() {
				restartNeeded := existingSettings.MergeSettings(UpdateSettings{
					Mbus: MBus{
						Cert: CertKeyPair{
							CA: "new CA",
						},
					},
				})
				Expect(restartNeeded).To(BeTrue())
				Expect(existingSettings.Mbus.Cert.CA).To(Equal("new CA"))
			})
		})

		Context("when the existing update settings json contains blobstore settings", func() {
			BeforeEach(func() {
				existingSettings = UpdateSettings{
					Blobstores: []Blobstore{
						{Type: "existing blobstore"},
					},
				}
			})

			It("does not replace the existing settings with empty values", func() {
				restartNeeded := existingSettings.MergeSettings(UpdateSettings{})
				Expect(restartNeeded).To(BeFalse())
				Expect(existingSettings.Blobstores[0].Type).To(Equal("existing blobstore"))
			})

			It("updates blobstore settings with new values", func() {
				restartNeeded := existingSettings.MergeSettings(UpdateSettings{
					Blobstores: []Blobstore{
						{Type: "new blobstore"},
					},
				})
				Expect(restartNeeded).To(BeTrue())
				Expect(existingSettings.Blobstores[0].Type).To(Equal("new blobstore"))
			})
		})
	})
})
