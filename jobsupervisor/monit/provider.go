package monit

import (
	"net/http"
	"time"

	bosherr "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/errors"
	boshhttp "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/http"
	boshlog "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/pivotal-golang/clock"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
)

const (
	shortRetryStrategyAttempts = uint(20)
	longRetryStrategyAttempts  = uint(300)
	retryDelay                 = 1 * time.Second
	monitHost                  = "127.0.0.1:2822"
)

type ClientProvider interface {
	Get() (Client, error)
}

type clientProvider struct {
	platform        boshplatform.Platform
	logger          boshlog.Logger
	shortHTTPClient boshhttp.Client
	longHTTPClient  boshhttp.Client
}

func NewProvider(platform boshplatform.Platform, logger boshlog.Logger) ClientProvider {
	httpClient := http.DefaultClient

	shortHTTPClient := boshhttp.NewRetryClient(
		httpClient,
		shortRetryStrategyAttempts,
		retryDelay,
		logger,
	)

	longHTTPClient := NewMonitRetryClient(
		httpClient,
		longRetryStrategyAttempts,
		shortRetryStrategyAttempts,
		retryDelay,
		logger,
	)

	return clientProvider{
		platform:        platform,
		logger:          logger,
		shortHTTPClient: shortHTTPClient,
		longHTTPClient:  longHTTPClient,
	}
}

func (p clientProvider) Get() (client Client, err error) {
	monitUser, monitPassword, err := p.platform.GetMonitCredentials()
	timeService := clock.NewClock()
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
		timeService,
	), nil
}
