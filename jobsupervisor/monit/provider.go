package monit

import (
	"net/http"
	"time"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshtime "github.com/cloudfoundry/bosh-agent/time"
)

type clientProvider struct {
	platform    boshplatform.Platform
	logger      boshlog.Logger
	timeService boshtime.Service
}

func NewProvider(platform boshplatform.Platform, logger boshlog.Logger, timeService boshtime.Service) clientProvider {
	return clientProvider{platform: platform, logger: logger, timeService: timeService}
}

func (p clientProvider) Get() (client Client, err error) {
	monitUser, monitPassword, err := p.platform.GetMonitCredentials()
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting monit credentials")
	}

	return NewHTTPClient(
		"127.0.0.1:2822",
		monitUser,
		monitPassword,
		http.DefaultClient,
		1*time.Second,
		1*time.Second,
		1*time.Second,
		20,
		300,
		300,
		p.logger,
		p.timeService,
	), nil
}
