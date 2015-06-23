package monit_test

import (
	"errors"
	"net/http"
	"time"

	boshhttp "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/http"
	fakehttp "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/http/fakes"
	boshretry "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/retrystrategy"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/pivotal-golang/clock/fakeclock"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
	"io"
)

var _ = Describe("MonitRetryStrategy", func() {
	var (
		retryable              *fakehttp.FakeRequestRetryable
		monitRetryStrategy     boshretry.RetryStrategy
		maxUnavailableAttempts int
		maxOtherAttempts       int
		timeService            *fakeclock.FakeClock
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
		timeService = fakeclock.NewFakeClock(time.Now())
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
			lastError   error
			unavailable *http.Response
			notFound    *http.Response
		)

		BeforeEach(func() {
			lastError = errors.New("last-error")
			unavailable = &http.Response{StatusCode: 503, Body: boshhttp.NewStringReadCloser("")}
			notFound = &http.Response{StatusCode: 404, Body: boshhttp.NewStringReadCloser("")}
		})

		Context("when all responses are only 503s", func() {
			It("retries until maxUnavailableAttempts + maxOtherAttempts are exhausted", func() {
				for i := 0; i < maxUnavailableAttempts+maxOtherAttempts-1; i++ {
					retryable.AddAttemptBehavior(unavailable, true, errors.New("fake-error"))
				}
				retryable.AddAttemptBehavior(unavailable, true, lastError)

				errChan := tryInBackground(monitRetryStrategy)

				sleepForIncrements(timeService, maxUnavailableAttempts+maxOtherAttempts, delay)

				Eventually(errChan).Should(Receive(Equal(lastError)))
				Expect(retryable.AttemptCalled).To(Equal(maxUnavailableAttempts + maxOtherAttempts))
			})
		})

		Context("when there are < maxUnavailableAttempts initial 503s", func() {
			var expectedAttempts int

			BeforeEach(func() {
				expectedAttempts = maxUnavailableAttempts + maxOtherAttempts - 1
				for i := 0; i < maxUnavailableAttempts-1; i++ {
					retryable.AddAttemptBehavior(unavailable, true, errors.New("unavailable-error"))
				}
			})

			Context("when maxOtherAttempts non-503 errors", func() {
				It("retries the unavailable then until maxOtherAttempts are exhausted", func() {
					for i := 0; i < maxOtherAttempts-1; i++ {
						retryable.AddAttemptBehavior(notFound, true, errors.New("not-found-error"))
					}
					retryable.AddAttemptBehavior(notFound, true, lastError)

					errChan := tryInBackground(monitRetryStrategy)

					sleepForIncrements(timeService, expectedAttempts, delay)

					Eventually(errChan).Should(Receive(Equal(lastError)))
					Expect(retryable.AttemptCalled).To(Equal(expectedAttempts))
				})
			})

			Context("when maxOtherAttempts include 503s after non-503", func() {
				It("retries the unavailable then until maxOtherAttempts are exhausted", func() {
					retryable.AddAttemptBehavior(notFound, true, errors.New("not-found-error"))
					for i := 0; i < maxOtherAttempts-2; i++ {
						retryable.AddAttemptBehavior(unavailable, true, errors.New("unavailable-error"))
					}
					retryable.AddAttemptBehavior(unavailable, true, lastError)

					errChan := tryInBackground(monitRetryStrategy)

					sleepForIncrements(timeService, expectedAttempts, delay)

					Eventually(errChan).Should(Receive(Equal(lastError)))
					Expect(retryable.AttemptCalled).To(Equal(expectedAttempts))
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

				errChan := tryInBackground(monitRetryStrategy)

				sleepForIncrements(timeService, maxOtherAttempts, delay)

				Eventually(errChan).Should(Receive(Equal(lastError)))
				Expect(retryable.AttemptCalled).To(Equal(maxOtherAttempts))
			})

			Context("when other attempts are all unavailble", func() {
				It("retries for maxOtherAttempts", func() {
					for i := 0; i < maxOtherAttempts-2; i++ {
						retryable.AddAttemptBehavior(unavailable, true, errors.New("unavailable-error"))
					}
					retryable.AddAttemptBehavior(unavailable, true, lastError)

					errChan := tryInBackground(monitRetryStrategy)

					sleepForIncrements(timeService, maxOtherAttempts, delay)

					Eventually(errChan).Should(Receive(Equal(lastError)))
					Expect(retryable.AttemptCalled).To(Equal(maxOtherAttempts))
				})

				It("closes the response body", func() {
					for i := 0; i < maxOtherAttempts-2; i++ {
						retryable.AddAttemptBehavior(unavailable, true, errors.New("unavailable-error"))
					}
					retryable.AddAttemptBehavior(unavailable, true, lastError)

					errChan := tryInBackground(monitRetryStrategy)

					sleepForIncrements(timeService, maxOtherAttempts, delay)

					Eventually(errChan).Should(Receive(Equal(lastError)))
					Expect(retryable.Response().Body.(ClosedChecker).Closed()).To(BeTrue())
				})
			})
		})

		It("waits for retry delay between retries", func() {
			for i := 0; i < maxUnavailableAttempts+maxOtherAttempts; i++ {
				retryable.AddAttemptBehavior(unavailable, true, nil)
			}

			go func() {
				monitRetryStrategy.Try()
			}()

			Eventually(retryable.Attempts).Should(Equal(1))
			timeService.Increment(delay - time.Millisecond)
			Consistently(retryable.Attempts).Should(Equal(1))
			timeService.Increment(2 * time.Millisecond)
			Eventually(retryable.Attempts).Should(Equal(2))
		})

		Context("when error is not due to failed response", func() {
			It("retries until maxOtherAttempts are exhausted", func() {
				for i := 0; i < maxOtherAttempts-1; i++ {
					retryable.AddAttemptBehavior(nil, true, errors.New("request error"))
				}
				retryable.AddAttemptBehavior(nil, true, lastError)

				errChan := tryInBackground(monitRetryStrategy)

				sleepForIncrements(timeService, maxOtherAttempts, delay)

				Eventually(errChan).Should(Receive(Equal(lastError)))
				Expect(retryable.Attempts()).To(Equal(maxOtherAttempts))
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

func tryInBackground(monitRetryStrategy boshretry.RetryStrategy) chan error {
	errChan := make(chan error)
	go func() {
		errChan <- monitRetryStrategy.Try()
	}()
	return errChan
}

func sleepForIncrements(timeService *fakeclock.FakeClock, attempts int, delay time.Duration) {
	for i := 0; i < attempts; i++ {
		Eventually(timeService.WatcherCount).Should(Equal(1))
		timeService.Increment(delay)
		Eventually(timeService.WatcherCount).Should(Equal(0))
	}
}
