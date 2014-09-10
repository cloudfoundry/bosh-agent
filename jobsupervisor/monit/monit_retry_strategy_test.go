package monit_test

import (
	"errors"
	"net/http"

	. "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshhttp "github.com/cloudfoundry/bosh-agent/http"
)

var _ = Describe("MonitRetryStrategy", func() {
	var (
		monitRetryStrategy     boshhttp.RetryStrategy
		monitRetryHandler      boshhttp.RetryHandler
		maxUnavailableAttempts uint
		maxOtherAttempts       uint
	)

	BeforeEach(func() {
		maxUnavailableAttempts = 2
		maxOtherAttempts = 3
		monitRetryStrategy = NewMonitRetryStrategy(maxUnavailableAttempts, maxOtherAttempts)
		monitRetryHandler = monitRetryStrategy.NewRetryHandler()
	})

	Describe("IsRetryable", func() {
		var (
			request  *http.Request
			response *http.Response
			err      error
		)

		BeforeEach(func() {
			request = &http.Request{}
			response = nil
			err = nil
		})

		It("retries until maxUnavailableAttempts + maxOtherAttempts are exhausted, if all attempts respond with 503", func() {
			response = &http.Response{
				StatusCode: 503,
			}
			for i := uint(0); i < maxUnavailableAttempts; i++ {
				Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeTrue())
			}

			for i := uint(0); i < maxOtherAttempts; i++ {
				Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeTrue())
			}

			Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeFalse())
		})

		It("503s after non-503s count as 'other attempts'", func() {
			response = &http.Response{
				StatusCode: 500,
			}

			Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeTrue())

			response = &http.Response{
				StatusCode: 503,
			}

			for i := uint(0); i < maxOtherAttempts-1; i++ {
				Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeTrue())
			}

			Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeFalse())
		})

		It("503s after errors count as 'other attempts'", func() {
			err = errors.New("fake-error")

			Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeTrue())

			response = &http.Response{
				StatusCode: 503,
			}
			err = nil

			for i := uint(0); i < maxOtherAttempts-1; i++ {
				Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeTrue())
			}

			Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeFalse())
		})

		It("retries until maxOtherAttempts are exhausted, if all attempts respond with non-503", func() {
			response = &http.Response{
				StatusCode: 500,
			}

			for i := uint(0); i < maxOtherAttempts; i++ {
				Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeTrue())
			}

			Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeFalse())
		})

		It("retries until maxOtherAttempts are exhausted, if a previous attempt responded with 503", func() {
			response = &http.Response{
				StatusCode: 503,
			}

			for i := uint(0); i < maxUnavailableAttempts-1; i++ {
				Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeTrue())
			}

			response = &http.Response{
				StatusCode: 500,
			}

			for i := uint(0); i < maxOtherAttempts; i++ {
				Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeTrue())
			}

			Expect(monitRetryHandler.IsRetryable(request, response, err)).To(BeFalse())
		})

		It("retries until maxOtherAttempts are exhausted, if all attemtps error", func() {
			err = errors.New("fake-error")

			for i := uint(0); i < maxOtherAttempts; i++ {
				Expect(monitRetryHandler.IsRetryable(nil, nil, err)).To(BeTrue())
			}

			Expect(monitRetryHandler.IsRetryable(nil, nil, err)).To(BeFalse())
		})

	})
})
