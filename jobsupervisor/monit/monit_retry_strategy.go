package monit

import (
	"time"

	boshhttp "github.com/cloudfoundry/bosh-agent/http"
	boshretry "github.com/cloudfoundry/bosh-agent/retrystrategy"
	boshtime "github.com/cloudfoundry/bosh-agent/time"
)

type monitRetryStrategy struct {
	retryable boshhttp.RequestRetryable

	maxUnavailableAttempts uint
	maxOtherAttempts       uint

	delay       time.Duration
	timeService boshtime.Service

	unavailableAttempts uint
	otherAttempts       uint
}

func NewMonitRetryStrategy(
	retryable boshhttp.RequestRetryable,
	maxUnavailableAttempts uint,
	maxOtherAttempts uint,
	delay time.Duration,
	timeService boshtime.Service,
) boshretry.RetryStrategy {
	return &monitRetryStrategy{
		retryable:              retryable,
		maxUnavailableAttempts: maxUnavailableAttempts,
		maxOtherAttempts:       maxOtherAttempts,
		unavailableAttempts:    0,
		otherAttempts:          0,
		delay:                  delay,
		timeService:            timeService,
	}
}

func (m *monitRetryStrategy) Try() error {
	var err error
	var isRetryable bool

	for m.hasMoreAttempts() {
		isRetryable, err = m.retryable.Attempt()
		if !isRetryable {
			break
		}

		if m.retryable.Response() != nil && m.retryable.Response().StatusCode == 503 && m.unavailableAttempts < m.maxUnavailableAttempts {
			m.unavailableAttempts = m.unavailableAttempts + 1
		} else {
			// once a non-503 error is received, all errors count as 'other' errors
			m.unavailableAttempts = m.maxUnavailableAttempts + 1
			m.otherAttempts = m.otherAttempts + 1
		}

		m.timeService.Sleep(m.delay)
	}

	if err != nil && m.retryable.Response() != nil {
		m.retryable.Response().Body.Close()
	}

	return err
}

func (m *monitRetryStrategy) hasMoreAttempts() bool {
	return m.unavailableAttempts < m.maxUnavailableAttempts || m.otherAttempts < m.maxOtherAttempts
}
