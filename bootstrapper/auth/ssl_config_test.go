package auth_test

import (
	"github.com/cloudfoundry/bosh-agent/bootstrapper/auth"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/spec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ssl config construction", func() {
	It("parses valid allowed names", func() {
		allowedNames := []string{"o=bosh.director"}
		_, err := auth.NewSSLConfig(
			spec.FixtureFilename("certs/bootstrapper.crt"),
			spec.FixtureFilename("certs/bootstrapper.key"),
			(string)(spec.FixtureData("certs/rootCA.pem")),
			allowedNames,
		)
		Expect(err).ToNot(HaveOccurred())
	})

	It("errors on malformed allowed names", func() {
		allowedNames := []string{"invalid=value"}
		_, err := auth.NewSSLConfig(
			spec.FixtureFilename("certs/bootstrapper.crt"),
			spec.FixtureFilename("certs/bootstrapper.key"),
			(string)(spec.FixtureData("certs/rootCA.pem")),
			allowedNames,
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Invalid AllowedNames: Unknown field 'invalid'"))
	})

	It("errors on empty allowed names", func() {
		allowedNames := []string{}
		_, err := auth.NewSSLConfig(
			spec.FixtureFilename("certs/bootstrapper.crt"),
			spec.FixtureFilename("certs/bootstrapper.key"),
			(string)(spec.FixtureData("certs/rootCA.pem")),
			allowedNames,
		)
		Expect(err).To(HaveOccurred())
	})
})
