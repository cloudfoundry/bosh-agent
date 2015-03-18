package kickstart_test

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net/http"

	. "github.com/cloudfoundry/bosh-agent/kickstart"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("kickstart auth", authDesc)

func authDesc() {
	Describe(".ParseDistinguishedName", func() {
		It("decodes parts of a dn", func() {
			name, err := ParseDistinguishedName("C=US,O=Cloud Foundry,OU=4th Floor,ST=California,CN=Joe,L=San Francisco,SERIALNUMBER=1234")
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
			name, err := ParseDistinguishedName("c=US,o=Cloud Foundry,ou=4th Floor,st=California,cn=Joe,l=San Francisco,serialNumber=1234")
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
			_, err := ParseDistinguishedName("cz=US,cn=Whatever")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Unknown field 'cz'"))

			_, err = ParseDistinguishedName("cn=Whatever,cz=US")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Unknown field 'cz'"))
		})

		It("reports strange sequences as errors", func() {
			_, err := ParseDistinguishedName("x,")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Unknown field ''"))
		})

		It("honors escaped commas", func() {
			name, err := ParseDistinguishedName("c=US,o=Cloud Foundry\\, Inc.")
			Expect(err).ToNot(HaveOccurred())
			Expect(*name).To(BeEquivalentTo(pkix.Name{
				Country:      []string{"US"},
				Organization: []string{"Cloud Foundry, Inc."},
			}))
		})

		It("handles multibyte characters", func() {
			name, err := ParseDistinguishedName("o=Cloüd Foundry")
			Expect(err).ToNot(HaveOccurred())
			Expect(*name).To(BeEquivalentTo(pkix.Name{
				Organization: []string{"Cloüd Foundry"},
			}))
		})

		It("handles emptpy string", func() {
			name, err := ParseDistinguishedName("")
			Expect(err).ToNot(HaveOccurred())
			Expect(*name).To(BeEquivalentTo(pkix.Name{}))
		})
	})

	Describe("DNPatterns", func() {
		Describe(".Verify", func() {
			It("returns an error if no certificate was provided", func() {
				patterns, err := ParseDistinguishedNames([]string{"o=nonmatch", "o=match", "o=noway"})
				Expect(err).ToNot(HaveOccurred())
				err = patterns.Verify(&http.Request{})
				Expect(err).To(HaveOccurred())
				err = patterns.Verify(&http.Request{TLS: &tls.ConnectionState{}})
				Expect(err).To(HaveOccurred())
			})

			It("returns no error if the subject of the certificate matches in the list of distinguished names", func() {
				patterns, err := ParseDistinguishedNames([]string{"o=nonmatch", "o=match", "o=noway"})
				Expect(err).ToNot(HaveOccurred())
				err = patterns.Verify(&http.Request{
					TLS: &tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{
							&x509.Certificate{Subject: pkix.Name{Organization: []string{"match"}}},
						},
					},
				})
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns an error if the subject doesn't match any of the allowed DNs", func() {
				patterns, err := ParseDistinguishedNames([]string{"o=nonmatch", "o=noway"})
				Expect(err).ToNot(HaveOccurred())
				err = patterns.Verify(&http.Request{
					TLS: &tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{
							&x509.Certificate{Subject: pkix.Name{Organization: []string{"match"}}},
						},
					},
				})
				Expect(err).To(HaveOccurred())
			})

			It("returns no error if configured to match all DNs", func() {
				patterns, err := ParseDistinguishedNames([]string{"o=nonmatch", "*"})
				Expect(err).ToNot(HaveOccurred())
				err = patterns.Verify(&http.Request{
					TLS: &tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{
							&x509.Certificate{Subject: pkix.Name{Organization: []string{"match"}}},
						},
					},
				})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe(".VerifyOne", func() {
			parseDN := func(dn string) *pkix.Name {
				name, err := ParseDistinguishedName(dn)
				Expect(err).ToNot(HaveOccurred())
				return name
			}

			name := parseDN("c=US,o=org,ou=org-unit,l=SF,st=CA,serialNumber=1234,cn=common")

			It("compares a pkix.Name to a pattern", func() {
				Expect(MatchName(parseDN("c=US,o=org,ou=org-unit,l=SF,st=CA,serialNumber=1234,cn=common"),
					name)).To(BeTrue())

				Expect(MatchName(&pkix.Name{}, name)).To(BeTrue())
				Expect(MatchName(parseDN("cn=common"), name)).To(BeTrue())
				Expect(MatchName(parseDN("cn=co*on"), name)).To(BeTrue())
				Expect(MatchName(parseDN("c=xxx*"), name)).To(BeFalse())
				Expect(MatchName(parseDN("o=xxx*"), name)).To(BeFalse())
				Expect(MatchName(parseDN("ou=xxx*"), name)).To(BeFalse())
				Expect(MatchName(parseDN("l=xxx*"), name)).To(BeFalse())
				Expect(MatchName(parseDN("st=xxx*"), name)).To(BeFalse())
				Expect(MatchName(parseDN("serialNumber=xxx*"), name)).To(BeFalse())
				Expect(MatchName(parseDN("cn=xxx*"), name)).To(BeFalse())

				Expect(MatchName(parseDN("c=US,o=org,ou=org-*,l=SF,st=CA,serialNumber=1234,cn=common"),
					name)).To(BeTrue())
				Expect(MatchName(parseDN("c=US,o=org,ou=org-*,l=Oakland,st=CA,serialNumber=1234,cn=common"),
					name)).To(BeFalse())
				Expect(MatchName(parseDN("c=*,o=*,ou=*,l=*,st=*,serialNumber=*,cn=*"),
					name)).To(BeTrue())
			})
		})
	})
}
