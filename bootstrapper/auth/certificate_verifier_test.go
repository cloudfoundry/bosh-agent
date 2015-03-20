package auth_test

import (
	"crypto/x509"
	"crypto/x509/pkix"

	. "github.com/cloudfoundry/bosh-agent/bootstrapper/auth"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("bootstrapper auth", certificateVerifierDesc)

func certificateVerifierDesc() {
	Describe("CertificateVerifier", func() {
		Describe(".Verify", func() {
			It("returns an error if no certificate was provided", func() {
				verifier := &CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{Organization: []string{"nonmatch"}},
					pkix.Name{Organization: []string{"match"}},
					pkix.Name{Organization: []string{"noway"}},
				}}
				certificate := []*x509.Certificate{}

				err := verifier.Verify(certificate)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("No peer certificates"))
			})

			It("returns no error if the subject of the certificate matches in the list of distinguished names", func() {
				verifier := &CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{Organization: []string{"nonmatch"}},
					pkix.Name{Organization: []string{"match"}},
					pkix.Name{Organization: []string{"noway"}},
				}}
				certificate := []*x509.Certificate{
					&x509.Certificate{Subject: pkix.Name{Organization: []string{"match"}}},
				}
				Expect(verifier.Verify(certificate)).ToNot(HaveOccurred())
			})

			It("returns an error if the subject doesn't match any of the allowed DNs", func() {
				verifier := &CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{Organization: []string{"nonmatch"}},
					pkix.Name{Organization: []string{"noway"}},
				}}
				certificate := []*x509.Certificate{
					&x509.Certificate{Subject: pkix.Name{Organization: []string{"match"}}},
				}
				Expect(verifier.Verify(certificate)).To(HaveOccurred())
			})

			It("returns no error if configured to match all DNs", func() {
				verifier := &CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{Organization: []string{"nonmatch"}},
					pkix.Name{},
				}}
				certificate := []*x509.Certificate{
					&x509.Certificate{Subject: pkix.Name{Organization: []string{"match"}}},
				}
				Expect(verifier.Verify(certificate)).ToNot(HaveOccurred())
			})

			It("compares a pkix.Name to a pattern", func() {
				certificate := []*x509.Certificate{
					&x509.Certificate{
						Subject: pkix.Name{
							Country:            []string{"US"},
							Organization:       []string{"org"},
							OrganizationalUnit: []string{"org-unit"},
							Locality:           []string{"SF"},
							Province:           []string{"CA"},
							SerialNumber:       "1234",
							CommonName:         "common",
						},
					},
				}

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{
						Country:            []string{"US"},
						Organization:       []string{"org"},
						OrganizationalUnit: []string{"org-unit"},
						Locality:           []string{"SF"},
						Province:           []string{"CA"},
						SerialNumber:       "1234",
						CommonName:         "common",
					},
				}}).Verify(certificate)).ToNot(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{pkix.Name{}}}).Verify(certificate)).ToNot(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{CommonName: "common"},
				}}).Verify(certificate)).ToNot(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{CommonName: "co*on"},
				}}).Verify(certificate)).ToNot(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{Country: []string{"xxx*"}},
				}}).Verify(certificate)).To(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{Organization: []string{"xxx*"}},
				}}).Verify(certificate)).To(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{OrganizationalUnit: []string{"xxx*"}},
				}}).Verify(certificate)).To(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{Locality: []string{"xxx*"}},
				}}).Verify(certificate)).To(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{Province: []string{"xxx*"}},
				}}).Verify(certificate)).To(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{SerialNumber: "xxx*"},
				}}).Verify(certificate)).To(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{CommonName: "xxx*"},
				}}).Verify(certificate)).To(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{
						Country:            []string{"US"},
						Organization:       []string{"org"},
						OrganizationalUnit: []string{"org-*"},
						Locality:           []string{"SF"},
						Province:           []string{"CA"},
						SerialNumber:       "1234",
						CommonName:         "common",
					},
				}}).Verify(certificate)).ToNot(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{
						Country:            []string{"US"},
						Organization:       []string{"org"},
						OrganizationalUnit: []string{"org-*"},
						Locality:           []string{"Oakland"},
						Province:           []string{"CA"},
						SerialNumber:       "1234",
						CommonName:         "common",
					},
				}}).Verify(certificate)).To(HaveOccurred())

				Expect((&CertificateVerifier{AllowedNames: []pkix.Name{
					pkix.Name{
						Country:            []string{"*"},
						Organization:       []string{"*"},
						OrganizationalUnit: []string{"*"},
						Locality:           []string{"*"},
						Province:           []string{"*"},
						SerialNumber:       "*",
						CommonName:         "*",
					},
				}}).Verify(certificate)).ToNot(HaveOccurred())
			})
		})
	})
}
