package mbus

import (
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"github.com/cloudfoundry/bosh-agent/v2/agentpassword"
	"github.com/cloudfoundry/bosh-agent/v2/settings"

	tlsconfig "code.cloudfoundry.org/tlsconfig"
)

const (
	httpsDispatcherLogTag = "HTTPS Dispatcher"
	maxHTTPPasswordLength = 1024
)

type HTTPSDispatcher struct {
	httpServer            *http.Server
	mux                   *http.ServeMux
	keyPair               settings.CertKeyPair
	listener              net.Listener
	logger                boshlog.Logger
	baseURL               *url.URL
	passwordVerifier      *agentpassword.Verifier
	expectedAuthorization string
}

type HTTPHandlerFunc func(writer http.ResponseWriter, request *http.Request)

func NewHTTPSDispatcher(baseURL *url.URL, keyPair settings.CertKeyPair, logger boshlog.Logger) (*HTTPSDispatcher, error) {
	password, _ := baseURL.User.Password()
	var verifier *agentpassword.Verifier
	var err error
	if agentpassword.IsVerifier(password) {
		verifier, err = agentpassword.ParseVerifier(password)
		if err != nil {
			return nil, bosherr.WrapError(err, "Configuring HTTP authentication")
		}
	}

	tlsConfig, _ := tlsconfig.Build(tlsconfig.WithInternalServiceDefaults()).Server() //nolint:errcheck

	httpServer := &http.Server{
		TLSConfig: tlsConfig,
	}
	mux := http.NewServeMux()
	httpServer.Handler = mux

	auth := fmt.Sprintf("%s:%s", baseURL.User.Username(), password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))

	return &HTTPSDispatcher{
		httpServer:            httpServer,
		mux:                   mux,
		keyPair:               keyPair,
		logger:                logger,
		baseURL:               baseURL,
		passwordVerifier:      verifier,
		expectedAuthorization: fmt.Sprintf("Basic %s", encodedAuth),
	}, nil
}

func (h *HTTPSDispatcher) Start() error {
	tcpListener, err := net.Listen("tcp", h.baseURL.Host)
	if err != nil {
		return bosherr.WrapError(err, "Starting HTTP listener")
	}
	h.listener = tcpListener

	var cert tls.Certificate
	cert, err = tls.X509KeyPair([]byte(h.keyPair.Certificate), []byte(h.keyPair.PrivateKey))
	if err != nil {
		return bosherr.WrapError(err, "Loading configured tls certificate")
	}

	// update the server config with the cert
	config := h.httpServer.TLSConfig
	config.NextProtos = []string{"http/1.1"}
	config.Certificates = []tls.Certificate{cert}

	tlsListener := tls.NewListener(tcpListener, config)

	return h.httpServer.Serve(tlsListener)
}

func (h *HTTPSDispatcher) Stop() {
	if h.listener != nil {
		_ = h.listener.Close() //nolint:errcheck
		h.listener = nil
	}
}

// requestNotAuthorized checks HTTP credentials; cryptographic verification lives
// in agentpassword.Verifier and is safe for simultaneous requests.
func (h *HTTPSDispatcher) requestNotAuthorized(request *http.Request) bool {
	if h.passwordVerifier == nil {
		return subtle.ConstantTimeCompare([]byte(h.expectedAuthorization), []byte(request.Header.Get("Authorization"))) != 1
	}
	username, password, ok := request.BasicAuth()
	return !ok || subtle.ConstantTimeCompare([]byte(h.baseURL.User.Username()), []byte(username)) != 1 ||
		len(password) == 0 || len(password) > maxHTTPPasswordLength || !h.passwordVerifier.Matches(password)
}

func (h *HTTPSDispatcher) AddRoute(route string, handler HTTPHandlerFunc) {
	authWrapper := func(w http.ResponseWriter, r *http.Request) {
		h.logger.Info(httpsDispatcherLogTag, fmt.Sprintf("%s %s", r.Method, r.URL.Path))

		if h.requestNotAuthorized(r) {
			w.Header().Add("WWW-Authenticate", `Basic realm=""`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}

	h.mux.HandleFunc(route, authWrapper)
}
