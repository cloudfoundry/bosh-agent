package listener

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"sync"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/auth"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/installer"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
)

type Listener struct {
	config    auth.SSLConfig
	installer installer.Installer
	server    http.Server
	listener  net.Listener
	started   bool
	closing   bool
	wg        sync.WaitGroup
}

func NewListener(config auth.SSLConfig, installer installer.Installer) *Listener {
	return &Listener{
		config:    config,
		installer: installer,
	}
}

func (l *Listener) ListenAndServe(logger logger.Logger, port int) error {
	certificateVerifier := &auth.CertificateVerifier{AllowedNames: l.config.PkixNames}

	serveMux := http.NewServeMux()
	updateHandler := &SelfUpdateHandler{Logger: logger, installer: l.installer}
	sslUpdateHandler := NewSSLHandler(logger, updateHandler, certificateVerifier)
	serveMux.Handle("/self-update", sslUpdateHandler)

	l.server.Handler = serveMux

	serverCert, err := tls.LoadX509KeyPair(l.config.CertFile, l.config.KeyFile)
	if err != nil {
		return err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(([]byte)(l.config.CACertPem)) {
		return errors.Errorf("Huh? root PEM looks weird!\n%s\n", l.config.CACertPem)
	}
	config := &tls.Config{
		NextProtos:   []string{"http/1.1"},
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		return err
	}

	l.listener = tls.NewListener(listener, config)

	l.started = true
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		err := l.server.Serve(l.listener)
		if err != nil && !l.closing {
			logger.Error("Listener", "unexpected server shutdown: %s", err)
		}
	}()

	return nil
}

func (l *Listener) Close() {
	if l.started {
		l.closing = true
		_ = l.listener.Close()
		l.started = false
	}
}

func (l *Listener) WaitForServerToExit() {
	l.wg.Wait()
}
