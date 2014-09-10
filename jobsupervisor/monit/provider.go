package monit

import (
	"net/http"
	"time"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshhttp "github.com/cloudfoundry/bosh-agent/http"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshtime "github.com/cloudfoundry/bosh-agent/time"
)

const (
	shortRetryStrategyAttempts = uint(20)
	longRetryStrategyAttempts  = uint(300)
	retryDelay                 = 1 * time.Second
	monitHost                  = "127.0.0.1:2822"
)

type clientProvider struct {
	platform        boshplatform.Platform
	logger          boshlog.Logger
	shortHTTPClient boshhttp.Client
	longHTTPClient  boshhttp.Client
}

func NewProvider(platform boshplatform.Platform, logger boshlog.Logger, timeService boshtime.Service) clientProvider {
	httpClient := http.DefaultClient

	shortRetryStrategy := boshhttp.NewAttemptRetryStrategy(shortRetryStrategyAttempts)
	shortHTTPClient := boshhttp.NewRetryClient(httpClient, shortRetryStrategy, retryDelay, timeService, logger)

	longRetryStrategy := NewMonitRetryStrategy(longRetryStrategyAttempts, shortRetryStrategyAttempts)
	longHTTPClient := boshhttp.NewRetryClient(httpClient, longRetryStrategy, retryDelay, timeService, logger)

	return clientProvider{
		platform:        platform,
		logger:          logger,
		shortHTTPClient: shortHTTPClient,
		longHTTPClient:  longHTTPClient,
	}
}

func (p clientProvider) Get() (client Client, err error) {
	monitUser, monitPassword, err := p.platform.GetMonitCredentials()
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting monit credentials")
	}

	return NewHTTPClient(
		monitHost,
		monitUser,
		monitPassword,
		p.shortHTTPClient,
		p.longHTTPClient,
		p.logger,
	), nil
}
