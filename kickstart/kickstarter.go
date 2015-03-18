package kickstart

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/cloudfoundry/bosh-agent/errors"
	"github.com/cloudfoundry/bosh-agent/logger"
)

type Kickstart struct {
	CertFile   string
	KeyFile    string
	CACertPem  string
	AllowedDNs []string

	Logger *log.Logger

	server   http.Server
	listener net.Listener
	started  bool
	closing  bool
	wg       sync.WaitGroup
}

const InstallScriptName = "install.sh"

func (k *Kickstart) Listen(port int) error {
	certAuthHandler, err := ParseDistinguishedNames(k.AllowedDNs)
	if err != nil {
		return err
	}

	serveMux := http.NewServeMux()
	logger := logger.New(logger.LevelDebug, k.Logger, k.Logger)
	serveMux.Handle("/self-update", certAuthHandler.WrapHandler(logger, &SelfUpdateHandler{Logger: logger}))

	k.server.Handler = serveMux
	k.server.ErrorLog = k.Logger

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		return err
	}

	serverCert, _ := tls.LoadX509KeyPair(k.CertFile, k.KeyFile)
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(([]byte)(k.CACertPem)) {
		return errors.Errorf("Huh? root PEM looks weird!\n%s\n", k.CACertPem)
	}
	config := &tls.Config{
		NextProtos:   []string{"http/1.1"},
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}
	k.listener = tls.NewListener(listener, config)

	k.wg.Add(1)
	go k.run()
	return nil
}

func (k *Kickstart) run() {
	defer k.wg.Done()
	k.started = true
	err := k.server.Serve(k.listener)
	if err != nil && !k.closing {
		fmt.Printf("run(): %s\n", err)
	}
}

func (k *Kickstart) WaitForServerToExit() {
	if k.started {
		k.closing = true
		k.listener.Close()
		k.started = false
	}

	k.wg.Wait()
}
