package monit_test

import (
	"errors"
	"net/http"
	"time"

	fakemonit "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit/fakes"
	boshretry "github.com/cloudfoundry/bosh-agent/retrystrategy"
	faketime "github.com/cloudfoundry/bosh-agent/time/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
)

var _ = Describe("MonitRetryStrategy", func() {
	var (
		retryable              *fakemonit.FakeMonitRetryable
		monitRetryStrategy     boshretry.RetryStrategy
		maxUnavailableAttempts int
		maxOtherAttempts       int
		timeService            *faketime.FakeService
		delay                  time.Duration
	)

	BeforeEach(func() {
		maxUnavailableAttempts = 6
		maxOtherAttempts = 7
		retryable = &fakemonit.FakeMonitRetryable{}
		timeService = &faketime.FakeService{}
		delay = 10 * time.Millisecond
		monitRetryStrategy = NewMonitRetryStrategy(
			retryable,
			uint(maxUnavailableAttempts),
			uint(maxOtherAttempts),
			delay,
			timeService,
		)
	})

	Describe("Try", func() {
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

		Context("when attempt is retryable", func() {
			BeforeEach(func() {
				retryable.AttemptIsRetryable = true
			})

			It("retries until maxUnavailableAttempts + maxOtherAttempts are exhausted, if all attempts respond with 503", func() {
				retryable.SetNextResponseStatus(503, maxUnavailableAttempts+maxOtherAttempts)

				err := monitRetryStrategy.Try()
				Expect(err).ToNot(HaveOccurred())

				Expect(retryable.AttemptCalled).To(Equal(maxUnavailableAttempts + maxOtherAttempts))
			})

			It("503s after non-503s count as 'other attempts'", func() {
				retryable.SetNextResponseStatus(500, 3)
				retryable.SetNextResponseStatus(503, maxOtherAttempts)

				err := monitRetryStrategy.Try()
				Expect(err).ToNot(HaveOccurred())

				Expect(retryable.AttemptCalled).To(Equal(maxOtherAttempts))
			})

			It("503s after errors count as 'other attempts'", func() {
				retryable.AttemptErrors = []error{errors.New("fake-error")}
				retryable.SetNextResponseStatus(503, maxOtherAttempts+maxUnavailableAttempts)

				err := monitRetryStrategy.Try()
				Expect(err).ToNot(HaveOccurred())

				Expect(retryable.AttemptCalled).To(Equal(maxOtherAttempts))
			})

			It("retries until maxOtherAttempts are exhausted, if all attempts respond with non-503", func() {
				retryable.SetNextResponseStatus(500, maxOtherAttempts+maxUnavailableAttempts)

				err := monitRetryStrategy.Try()
				Expect(err).ToNot(HaveOccurred())

				Expect(retryable.AttemptCalled).To(Equal(maxOtherAttempts))
			})

			It("retries until maxOtherAttempts are exhausted, if a previous attempt responded with 503", func() {
				unavailableAttempts := 2
				retryable.SetNextResponseStatus(503, unavailableAttempts)
				retryable.SetNextResponseStatus(500, 3)
				retryable.SetNextResponseStatus(503, maxOtherAttempts)

				err := monitRetryStrategy.Try()
				Expect(err).ToNot(HaveOccurred())

				Expect(retryable.AttemptCalled).To(Equal(maxOtherAttempts + unavailableAttempts))
			})

			It("retries until maxOtherAttempts are exhausted, if all attemtps error", func() {
				for i := 0; i < maxOtherAttempts; i++ {
					retryable.AttemptErrors = append(retryable.AttemptErrors, errors.New("fake-error"))
				}

				err := monitRetryStrategy.Try()
				Expect(err).ToNot(HaveOccurred())

				Expect(retryable.AttemptCalled).To(Equal(maxOtherAttempts))
			})

			It("waits for retry delay between retries", func() {
				retryable.SetNextResponseStatus(500, maxOtherAttempts)

				err := monitRetryStrategy.Try()
				Expect(err).ToNot(HaveOccurred())

				Expect(len(timeService.SleepInputs)).To(Equal(maxOtherAttempts))
				Expect(timeService.SleepInputs[0]).To(Equal(delay))
			})
		})

		Context("when attempt is not retryable", func() {
			BeforeEach(func() {
				retryable.AttemptIsRetryable = false
			})

			It("does not retry", func() {
				err := monitRetryStrategy.Try()
				Expect(err).ToNot(HaveOccurred())

				Expect(retryable.AttemptCalled).To(Equal(1))
			})
		})
	})
})
