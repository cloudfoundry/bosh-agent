package monit_test

import (
	"errors"
	"net/http"
	"time"

	boshhttp "github.com/cloudfoundry/bosh-agent/http"
	fakehttp "github.com/cloudfoundry/bosh-agent/http/fakes"
	boshretry "github.com/cloudfoundry/bosh-agent/retrystrategy"
	faketime "github.com/cloudfoundry/bosh-agent/time/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
	"io"
)

var _ = Describe("MonitRetryStrategy", func() {
	var (
		retryable              *fakehttp.FakeRequestRetryable
		monitRetryStrategy     boshretry.RetryStrategy
		maxUnavailableAttempts int
		maxOtherAttempts       int
		timeService            *faketime.FakeService
		delay                  time.Duration
	)

	type ClosedChecker interface {
		io.ReadCloser
		Closed() bool
	}

	BeforeEach(func() {
		maxUnavailableAttempts = 6
		maxOtherAttempts = 7
		retryable = fakehttp.NewFakeRequestRetryable()
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
			request     *http.Request
			response    *http.Response
			lastError   error
			err         error
			unavailable *http.Response
			notFound    *http.Response
		)

		BeforeEach(func() {
			request = &http.Request{}
			response = nil
			lastError = errors.New("last-error")
			err = nil
			unavailable = &http.Response{StatusCode: 503, Body: boshhttp.NewStringReadCloser("")}
			notFound = &http.Response{StatusCode: 404, Body: boshhttp.NewStringReadCloser("")}
		})

		Context("when all responses are only 503s", func() {
			It("retries until maxUnavailableAttempts + maxOtherAttempts are exhausted", func() {
				for i := 0; i < maxUnavailableAttempts+maxOtherAttempts-1; i++ {
					retryable.AddAttemptBehavior(unavailable, true, errors.New("fake-error"))
				}
				retryable.AddAttemptBehavior(unavailable, true, lastError)

				err := monitRetryStrategy.Try()
				Expect(err).To(Equal(lastError))

				Expect(retryable.AttemptCalled).To(Equal(maxUnavailableAttempts + maxOtherAttempts))
			})
		})

		Context("when there are < maxUnavailableAttempts initial 503s", func() {
			BeforeEach(func() {
				for i := 0; i < maxUnavailableAttempts-1; i++ {
					retryable.AddAttemptBehavior(unavailable, true, errors.New("unavailable-error"))
				}
			})

			Context("when maxOtherAttempts non-503 errors", func() {
				It("retries the unavailable then until maxOtherAttempts are exhasted", func() {
					for i := 0; i < maxOtherAttempts-1; i++ {
						retryable.AddAttemptBehavior(notFound, true, errors.New("not-found-error"))
					}
					retryable.AddAttemptBehavior(notFound, true, lastError)

					err := monitRetryStrategy.Try()
					Expect(err).To(Equal(lastError))

					Expect(retryable.AttemptCalled).To(Equal(maxUnavailableAttempts + maxOtherAttempts - 1))
				})
			})

			Context("when maxOtherAttempts include 503s after non-503", func() {
				It("retries the unavailable then until maxOtherAttempts are exhasted", func() {
					retryable.AddAttemptBehavior(notFound, true, errors.New("not-found-error"))
					for i := 0; i < maxOtherAttempts-2; i++ {
						retryable.AddAttemptBehavior(unavailable, true, errors.New("unavailable-error"))
					}
					retryable.AddAttemptBehavior(unavailable, true, lastError)

					err := monitRetryStrategy.Try()
					Expect(err).To(Equal(lastError))

					Expect(retryable.AttemptCalled).To(Equal(maxUnavailableAttempts + maxOtherAttempts - 1))
				})
			})
		})

		Context("when the initial attempt is a non-503 error", func() {
			BeforeEach(func() {
				retryable.AddAttemptBehavior(notFound, true, errors.New("not-found-error"))
			})

			It("retries for maxOtherAttempts", func() {
				for i := 0; i < maxOtherAttempts-2; i++ {
					retryable.AddAttemptBehavior(notFound, true, errors.New("not-found-error"))
				}
				retryable.AddAttemptBehavior(notFound, true, lastError)

				err := monitRetryStrategy.Try()
				Expect(err).To(Equal(lastError))

				Expect(retryable.AttemptCalled).To(Equal(maxOtherAttempts))
			})

			Context("when other attempts are all unavailble", func() {
				It("retries for maxOtherAttempts", func() {
					for i := 0; i < maxOtherAttempts-2; i++ {
						retryable.AddAttemptBehavior(unavailable, true, errors.New("unavailable-error"))
					}
					retryable.AddAttemptBehavior(unavailable, true, lastError)

					err := monitRetryStrategy.Try()
					Expect(err).To(Equal(lastError))

					Expect(retryable.AttemptCalled).To(Equal(maxOtherAttempts))
				})

				It("closes the response body", func() {
					for i := 0; i < maxOtherAttempts-2; i++ {
						retryable.AddAttemptBehavior(unavailable, true, errors.New("unavailable-error"))
					}
					retryable.AddAttemptBehavior(unavailable, true, lastError)

					err := monitRetryStrategy.Try()
					Expect(err).To(Equal(lastError))

					Expect(retryable.AttemptCalled).To(Equal(maxOtherAttempts))
					Expect(retryable.Response().Body.(ClosedChecker).Closed()).To(BeTrue())
				})
			})
		})

		It("waits for retry delay between retries", func() {
			for i := 0; i < maxUnavailableAttempts+maxOtherAttempts; i++ {
				retryable.AddAttemptBehavior(unavailable, true, nil)
			}

			monitRetryStrategy.Try()

			Expect(len(timeService.SleepInputs)).To(Equal(maxUnavailableAttempts + maxOtherAttempts))
			Expect(timeService.SleepInputs[0]).To(Equal(delay))
		})

		Context("when error is not due to failed response", func() {
			It("retries until maxOtherAttempts are exhausted", func() {
				for i := 0; i < maxOtherAttempts-1; i++ {
					retryable.AddAttemptBehavior(nil, true, errors.New("request error"))
				}
				retryable.AddAttemptBehavior(nil, true, lastError)

				err := monitRetryStrategy.Try()
				Expect(err).To(Equal(lastError))

				Expect(retryable.AttemptCalled).To(Equal(maxOtherAttempts))
			})
		})

		Context("when attempt is not retryable", func() {
			It("does not retry", func() {
				retryable.AddAttemptBehavior(nil, false, lastError)

				err := monitRetryStrategy.Try()
				Expect(err).To(Equal(lastError))

				Expect(retryable.AttemptCalled).To(Equal(1))
			})

			It("closes the response body", func() {
				retryable.AddAttemptBehavior(unavailable, false, lastError)
				err := monitRetryStrategy.Try()
				Expect(err).To(Equal(lastError))

				Expect(retryable.Response().Body.(ClosedChecker).Closed()).To(BeTrue())
			})
		})
	})
})
