package infrastructure_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
)

var _ = Describe("AwsMetadataServiceProvider", func() {
	var (
		awsMetadataServiceProvider MetadataServiceProvider
		fakeresolver               *fakeinf.FakeDNSResolver
	)

	BeforeEach(func() {
		fakeresolver = &fakeinf.FakeDNSResolver{}
		awsMetadataServiceProvider = NewAwsMetadataServiceProvider(fakeresolver)
	})

	Describe("Get", func() {
		It("returns http metadata service", func() {
			expectedMetadataService := NewHTTPMetadataService("http://169.254.169.254", fakeresolver)
			Expect(awsMetadataServiceProvider.Get()).To(Equal(expectedMetadataService))
		})
	})
})
