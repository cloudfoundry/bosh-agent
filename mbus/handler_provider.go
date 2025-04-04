package mbus

import (
	"net/url"

	"github.com/nats-io/nats.go"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	boshagentblobstore "github.com/cloudfoundry/bosh-agent/v2/agent/blobstore"
	boshhandler "github.com/cloudfoundry/bosh-agent/v2/handler"
	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type HandlerProvider struct {
	settingsService boshsettings.Service
	logger          boshlog.Logger
	auditLogger     boshplatform.AuditLogger
	handler         boshhandler.Handler
}

func NewHandlerProvider(
	settingsService boshsettings.Service,
	logger boshlog.Logger,
	auditLogger boshplatform.AuditLogger,
) (p HandlerProvider) {
	p.settingsService = settingsService
	p.logger = logger
	p.auditLogger = auditLogger
	return
}

func (p HandlerProvider) Get(
	platform boshplatform.Platform,
	blobManager boshagentblobstore.BlobManagerInterface,
) (boshhandler.Handler, error) {
	if p.handler != nil {
		return p.handler, nil
	}

	mbusURL, err := url.Parse(p.settingsService.GetSettings().GetMbusURL())
	if err != nil {
		return nil, bosherr.WrapError(err, "Parsing handler URL")
	}

	switch mbusURL.Scheme {
	case "nats":
		f := func(url string, options ...nats.Option) (NatsConnection, error) {
			return nats.Connect(url, options...)
		}
		return NewNatsHandler(p.settingsService, f, p.logger, platform), nil
	case "https":
		mbusKeyPair := p.settingsService.GetSettings().GetMbusCerts()
		return NewHTTPSHandler(mbusURL, mbusKeyPair, blobManager, p.logger, p.auditLogger), nil
	default:
		return nil, bosherr.Errorf("Message Bus Handler with scheme %s could not be found", mbusURL.Scheme)
	}
}
