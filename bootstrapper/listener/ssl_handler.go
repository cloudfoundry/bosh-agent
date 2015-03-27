package listener

import (
	"fmt"
	"net/http"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/auth"
	"github.com/cloudfoundry/bosh-agent/errors"
	"github.com/cloudfoundry/bosh-agent/logger"
)

type sslHandler struct {
	delegate            http.Handler
	logger              logger.Logger
	certificateVerifier *auth.CertificateVerifier
}

func NewSSLHandler(logger logger.Logger, delegate http.Handler, certificateVerifier *auth.CertificateVerifier) http.Handler {
	return &sslHandler{
		delegate:            delegate,
		logger:              logger,
		certificateVerifier: certificateVerifier,
	}
}

func (h *sslHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var err error
	if req.TLS == nil {
		err = errors.Error("Not SSL")
	}

	if err == nil {
		err = h.certificateVerifier.Verify(req.TLS.PeerCertificates)
	}

	if err != nil {
		rw.WriteHeader(http.StatusUnauthorized)
		h.logger.Error(fmt.Sprintf("%T", h), errors.WrapError(err, "Unauthorized access").Error())
		return
	}

	h.delegate.ServeHTTP(rw, req)
}
