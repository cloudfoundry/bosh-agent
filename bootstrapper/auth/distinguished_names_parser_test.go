package auth_test

import (
	"crypto/x509/pkix"
	. "github.com/cloudfoundry/bosh-agent/bootstrapper/auth"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("auth distinguishedNamesParser", dnpDesc)

func dnpDesc() {
	Describe("#Parse", func() {
		var parser DistinguishedNamesParser

		BeforeEach(func() {
			parser = NewDistinguishedNamesParser()
		})

		It("decodes parts of a dn", func() {
			name, err := parser.Parse("C=US,O=Cloud Foundry,OU=4th Floor,ST=California,CN=Joe,L=San Francisco,SERIALNUMBER=1234")
			Expect(err).ToNot(HaveOccurred())
			Expect(*name).To(BeEquivalentTo(pkix.Name{
				Country:            []string{"US"},
				Organization:       []string{"Cloud Foundry"},
				OrganizationalUnit: []string{"4th Floor"},
				Locality:           []string{"San Francisco"},
				Province:           []string{"California"},
				SerialNumber:       "1234",
				CommonName:         "Joe",
			}))
		})

		It("allows any case for identifiers", func() {
			name, err := parser.Parse("c=US,o=Cloud Foundry,ou=4th Floor,st=California,cn=Joe,l=San Francisco,serialNumber=1234")
			Expect(err).ToNot(HaveOccurred())
			Expect(*name).To(BeEquivalentTo(pkix.Name{
				Country:            []string{"US"},
				Organization:       []string{"Cloud Foundry"},
				OrganizationalUnit: []string{"4th Floor"},
				Locality:           []string{"San Francisco"},
				Province:           []string{"California"},
				SerialNumber:       "1234",
				CommonName:         "Joe",
			}))
		})

		It("reports unknown fields as errors", func() {
			_, err := parser.Parse("cz=US,cn=Whatever")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Unknown field 'cz'"))

			_, err = parser.Parse("cn=Whatever,cz=US")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Unknown field 'cz'"))
		})

		It("reports strange sequences as errors", func() {
			_, err := parser.Parse("x,")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Unknown field ''"))
		})

		It("empty string as errors", func() {
			_, err := parser.Parse("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Unknown field ''"))
		})

		It("reports strange sequences without commas as errors", func() {
			_, err := parser.Parse("foo")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Unknown field ''"))
		})

		It("honors escaped commas", func() {
			name, err := parser.Parse("c=US,o=Cloud Foundry\\, Inc.")
			Expect(err).ToNot(HaveOccurred())
			Expect(*name).To(BeEquivalentTo(pkix.Name{
				Country:      []string{"US"},
				Organization: []string{"Cloud Foundry, Inc."},
			}))
		})

		It("handles multibyte characters", func() {
			name, err := parser.Parse("o=Cloüd Foundry")
			Expect(err).ToNot(HaveOccurred())
			Expect(*name).To(BeEquivalentTo(pkix.Name{
				Organization: []string{"Cloüd Foundry"},
			}))
		})

		It("handles * string", func() {
			name, err := parser.Parse("*")
			Expect(err).ToNot(HaveOccurred())
			Expect(*name).To(BeEquivalentTo(pkix.Name{}))
		})
	})
}
