package monit

import (
	"net/http"

	boshhttp "github.com/cloudfoundry/bosh-agent/http"
)

type monitRetryStrategy struct {
	maxUnavailableAttempts uint
	maxOtherAttempts       uint
}

func NewMonitRetryStrategy(maxUnavailableAttempts, maxOtherAttempts uint) boshhttp.RetryStrategy {
	return &monitRetryStrategy{
		maxUnavailableAttempts: maxUnavailableAttempts,
		maxOtherAttempts:       maxOtherAttempts,
	}
}

func (m *monitRetryStrategy) NewRetryHandler() boshhttp.RetryHandler {
	return NewMonitRetryHandler(m.maxUnavailableAttempts, m.maxOtherAttempts)
}

type monitRetryHandler struct {
	unavailableAttempts    uint
	otherAttempts          uint
	maxUnavailableAttempts uint
	maxOtherAttempts       uint
}

func NewMonitRetryHandler(maxUnavailableAttempts, maxOtherAttempts uint) boshhttp.RetryHandler {
	return &monitRetryHandler{
		unavailableAttempts:    0,
		otherAttempts:          0,
		maxUnavailableAttempts: maxUnavailableAttempts,
		maxOtherAttempts:       maxOtherAttempts,
	}
}

func (m *monitRetryHandler) IsRetryable(_ *http.Request, resp *http.Response, err error) bool {
	if err == nil && resp.StatusCode == 503 && m.unavailableAttempts < m.maxUnavailableAttempts {
		m.unavailableAttempts = m.unavailableAttempts + 1
	} else {
		// once a non-503 error is received, all errors count as 'other' errors
		m.unavailableAttempts = m.maxUnavailableAttempts + 1
		m.otherAttempts = m.otherAttempts + 1
	}

	return m.unavailableAttempts <= m.maxUnavailableAttempts || m.otherAttempts <= m.maxOtherAttempts
}
