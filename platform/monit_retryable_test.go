package platform_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/platform"
	"github.com/cloudfoundry/bosh-agent/v2/servicemanager/servicemanagerfakes"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
)

var _ = Describe("MonitRetryable", func() {
	var (
		monitRetryable boshretry.Retryable
		serviceManager servicemanagerfakes.FakeServiceManager
	)

	BeforeEach(func() {
		serviceManager = servicemanagerfakes.FakeServiceManager{}
		monitRetryable = NewMonitRetryable(&serviceManager)
	})

	Describe("Attempt", func() {
		Context("when starting monit fails", func() {
			BeforeEach(func() {
				serviceManager.StartReturns(errors.New("fake-start-monit-error"))
			})

			It("is retryable and returns err", func() {
				shouldRetry, err := monitRetryable.Attempt()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-start-monit-error"))
				Expect(shouldRetry).To(BeTrue())
			})
		})

		Context("when starting succeeds", func() {
			It("returns no error", func() {
				_, err := monitRetryable.Attempt()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
