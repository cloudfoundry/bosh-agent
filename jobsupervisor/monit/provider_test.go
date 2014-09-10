package monit_test

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshhttp "github.com/cloudfoundry/bosh-agent/http"
	. "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	faketime "github.com/cloudfoundry/bosh-agent/time/fakes"
)

var _ = Describe("clientProvider", func() {
	It("Get", func() {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		platform := fakeplatform.NewFakePlatform()
		timeService := &faketime.FakeService{}

		platform.GetMonitCredentialsUsername = "fake-user"
		platform.GetMonitCredentialsPassword = "fake-pass"

		client, err := NewProvider(platform, logger, timeService).Get()
		Expect(err).ToNot(HaveOccurred())

		httpClient := http.DefaultClient

		shortRetryStrategy := boshhttp.NewAttemptRetryStrategy(20)
		shortHTTPClient := boshhttp.NewRetryClient(httpClient, shortRetryStrategy, 1*time.Second, timeService, logger)

		longRetryStrategy := NewMonitRetryStrategy(300, 20)
		longHTTPClient := boshhttp.NewRetryClient(httpClient, longRetryStrategy, 1*time.Second, timeService, logger)

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
