package monit_test

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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

		expectedClient := NewHTTPClient(
			"127.0.0.1:2822",
			"fake-user",
			"fake-pass",
			http.DefaultClient,
			1*time.Second,
			1*time.Second,
			1*time.Second,
			20,
			300,
			300,
			logger,
			timeService,
		)
		Expect(client).To(Equal(expectedClient))
	})
})
