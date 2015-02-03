package monit

import (
	"net/http"
	"time"

	boshretry "github.com/cloudfoundry/bosh-agent/retrystrategy"
	boshtime "github.com/cloudfoundry/bosh-agent/time"
)

type monitRetryable interface {
	Attempt() (bool, error)
	Response() *http.Response
}

type monitRetryStrategy struct {
	retryable monitRetryable

	maxUnavailableAttempts uint
	maxOtherAttempts       uint

	delay       time.Duration
	timeService boshtime.Service

	unavailableAttempts uint
	otherAttempts       uint
}

func NewMonitRetryStrategy(
	retryable monitRetryable,
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

	for m.isRetryable() {
		isRetryable, err := m.retryable.Attempt()
		if !isRetryable {
			return err
		}

		if err == nil && m.retryable.Response().StatusCode == 503 && m.unavailableAttempts < m.maxUnavailableAttempts {
			m.unavailableAttempts = m.unavailableAttempts + 1
		} else {
			// once a non-503 error is received, all errors count as 'other' errors
			m.unavailableAttempts = m.maxUnavailableAttempts + 1
			m.otherAttempts = m.otherAttempts + 1
		}

		m.timeService.Sleep(m.delay)
	}

	return err
}

func (m *monitRetryStrategy) isRetryable() bool {
	return m.unavailableAttempts < m.maxUnavailableAttempts || m.otherAttempts < m.maxOtherAttempts
}
