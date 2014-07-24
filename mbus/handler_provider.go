package mbus

import (
	"net/url"

	"github.com/cloudfoundry/yagnats"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	"github.com/cloudfoundry/bosh-agent/micro"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
)

type MbusHandlerProvider struct {
	settingsService boshsettings.Service
	logger          boshlog.Logger
	handler         boshhandler.Handler
}

func NewHandlerProvider(
	settingsService boshsettings.Service,
	logger boshlog.Logger,
) (p MbusHandlerProvider) {
	p.settingsService = settingsService
	p.logger = logger
	return
}

func (p MbusHandlerProvider) Get(
	platform boshplatform.Platform,
	dirProvider boshdir.DirectoriesProvider,
) (handler boshhandler.Handler, err error) {
	if p.handler != nil {
		handler = p.handler
		return
	}

	mbusURL, err := url.Parse(p.settingsService.GetSettings().Mbus)
	if err != nil {
		err = bosherr.WrapError(err, "Parsing handler URL")
		return
	}

	switch mbusURL.Scheme {
	case "nats":
		handler = NewNatsHandler(p.settingsService, yagnats.NewClient(), p.logger)
	case "https":
		handler = micro.NewHTTPSHandler(mbusURL, p.logger, platform.GetFs(), dirProvider)
	default:
		err = bosherr.New("Message Bus Handler with scheme %s could not be found", mbusURL.Scheme)
	}

	p.handler = handler

	return
}
