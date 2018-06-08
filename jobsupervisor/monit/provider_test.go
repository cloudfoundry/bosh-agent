package monit_test

import (
	"net/http"
	"time"

	. "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	"github.com/cloudfoundry/bosh-utils/httpclient"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("clientProvider", func() {
	var platform *platformfakes.FakePlatform

	It("Get", func() {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		platform = &platformfakes.FakePlatform{}

		platform.GetMonitCredentialsReturns("fake-user", "fake-pass", nil)

		client, err := NewProvider(platform, logger).Get()
		Expect(err).ToNot(HaveOccurred())

		httpClient := http.DefaultClient

		shortHTTPClient := httpclient.NewRetryClient(httpClient, 20, 1*time.Second, logger)
		longHTTPClient := NewMonitRetryClient(httpClient, 300, 20, 1*time.Second, logger)

		expectedClient := NewHTTPClient(
			"127.0.0.1:2822",
			"fake-user",
			"fake-pass",
			shortHTTPClient,
			longHTTPClient,
			logger,
		)
		Expect(client).To(Equal(expectedClient))
	})
})
