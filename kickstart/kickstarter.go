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
	CertFile  string
	KeyFile   string
	CACertPem string

	Logger *log.Logger
	wg     sync.WaitGroup
}

const InstallScriptName = "install.sh"

func (k *Kickstart) Listen(port int) error {
	serveMux := http.NewServeMux()
	serveMux.Handle("/self-update", &SelfUpdateHandler{
		Logger: logger.New(logger.LevelDebug, k.Logger, k.Logger),
	})

	server := &http.Server{
		Handler:  serveMux,
		ErrorLog: k.Logger,
	}

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
	tlsListener := tls.NewListener(listener, config)

	k.wg.Add(1)
	go k.run(server, tlsListener)
	return nil
}

func (k *Kickstart) run(server *http.Server, tlsListener net.Listener) {
	defer k.wg.Done()
	err := server.Serve(tlsListener)
	if err != nil {
		fmt.Printf("run(): %s\n", err)
	}
}

func (k *Kickstart) WaitForServerToExit() {
	k.wg.Wait()
}
