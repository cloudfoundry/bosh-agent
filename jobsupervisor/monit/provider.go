package monit

import (
	"net/http"
	"time"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
)

type clientProvider struct {
	platform boshplatform.Platform
	logger   boshlog.Logger
}

func NewProvider(platform boshplatform.Platform, logger boshlog.Logger) clientProvider {
	return clientProvider{platform: platform, logger: logger}
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
		p.logger,
	), nil
}
