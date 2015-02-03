package monit

import (
	"net/http"
	"time"

	boshhttp "github.com/cloudfoundry/bosh-agent/http"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshtime "github.com/cloudfoundry/bosh-agent/time"
)

type monitRetryClient struct {
	delegate               boshhttp.Client
	maxUnavailableAttempts uint
	maxOtherAttempts       uint
	retryDelay             time.Duration
	logger                 boshlog.Logger
}

func NewMonitRetryClient(
	delegate boshhttp.Client,
	maxUnavailableAttempts uint,
	maxOtherAttempts uint,
	retryDelay time.Duration,
	logger boshlog.Logger,
) boshhttp.Client {
	return &monitRetryClient{
		delegate:               delegate,
		maxUnavailableAttempts: maxUnavailableAttempts,
		maxOtherAttempts:       maxOtherAttempts,
		retryDelay:             retryDelay,
		logger:                 logger,
	}
}

func (r *monitRetryClient) Do(req *http.Request) (*http.Response, error) {
	requestRetryable := boshhttp.NewRequestRetryable(req, r.delegate, r.logger)
	timeService := boshtime.NewConcreteService()
	retryStrategy := NewMonitRetryStrategy(
		requestRetryable,
		r.maxUnavailableAttempts,
		r.maxOtherAttempts,
		r.retryDelay,
		timeService,
	)

	err := retryStrategy.Try()

	return requestRetryable.Response(), err
}
