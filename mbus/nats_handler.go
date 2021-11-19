package mbus

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"crypto/x509"
	"time"

	"crypto/tls"
	"regexp"

	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
)

const (
	responseMaxLength           = 1024 * 1024
	natsHandlerLogTag           = "NATS Handler"
	natsConnectionMaxRetries    = 10
	natsConnectRetryInterval    = 1 * time.Second
	natsConnectMaxRetryInterval = 1 * time.Minute
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

type Handler interface {
	Run(boshhandler.Func) error
	Start(boshhandler.Func) error
	RegisterAdditionalFunc(boshhandler.Func)
	Send(target boshhandler.Target, topic boshhandler.Topic, message interface{}) error
	Stop()
}

type NatsConnector func(url string, options ...nats.Option) (NatsConnection, error)

//counterfeiter:generate . NatsConnection

type NatsConnection interface {
	Close()
	Publish(subj string, data []byte) error
	Subscribe(subj string, cb nats.MsgHandler) (*nats.Subscription, error)
	//Subscribe(subject string, f func(natsMsg *nats.Msg)) (interface{}, error)
}

type natsHandler struct {
	settingsService boshsettings.Service
	connector       NatsConnector
	connection      NatsConnection
	platform        boshplatform.Platform

	handlerFuncs     []boshhandler.Func
	handlerFuncsLock sync.Mutex

	logger      boshlog.Logger
	auditLogger boshplatform.AuditLogger
	logTag      string

	connectRetryInterval    time.Duration
	maxConnectRetryInterval time.Duration
}

type ConnectionInfo struct {
	Addr      string
	IP        string
	Dial      func(network, address string) (net.Conn, error)
	TLSConfig *tls.Config
}

type ConnectionTLSInfo struct {
	CertPool              *x509.CertPool
	ClientCert            *tls.Certificate
	VerifyPeerCertificate func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error
}

func NewNatsHandler(
	settingsService boshsettings.Service,
	client NatsConnector,
	logger boshlog.Logger,
	platform boshplatform.Platform,
	connectRetryInterval time.Duration,
	maxConnectRetryInterval time.Duration,
) Handler {
	return &natsHandler{
		settingsService:         settingsService,
		connector:               client,
		platform:                platform,
		logger:                  logger,
		logTag:                  natsHandlerLogTag,
		auditLogger:             platform.GetAuditLogger(),
		connectRetryInterval:    connectRetryInterval,
		maxConnectRetryInterval: maxConnectRetryInterval,
	}
}

func (h *natsHandler) Run(handlerFunc boshhandler.Func) error {
	err := h.Start(handlerFunc)
	defer h.Stop()

	if err != nil {
		return bosherr.WrapError(err, "Starting nats handler")
	}

	h.runUntilInterrupted()

	return nil
}

func (h *natsHandler) Start(handlerFunc boshhandler.Func) error {
	h.RegisterAdditionalFunc(handlerFunc)

	connectionInfo, err := h.getConnectionInfo()
	if err != nil {
		return bosherr.WrapError(err, "Getting connection info")
	}

	natsRetryable := boshretry.NewRetryable(func() (bool, error) {
		h.logger.Info(h.logTag, "Attempting to connect to NATS")

		if net.ParseIP(connectionInfo.IP) != nil {
			err = h.platform.DeleteARPEntryWithIP(connectionInfo.IP)
			if err != nil {
				h.logger.Error(h.logTag, "Cleaning ip-mac address cache for: %s", connectionInfo.IP)
			}
		}

		var natsOptions []nats.Option
		if connectionInfo.TLSConfig != nil {
			natsOptions = append(natsOptions, nats.Secure(connectionInfo.TLSConfig))
		}

		connection, err := h.connector(connectionInfo.Addr, natsOptions...)
		if err != nil {
			return true, bosherr.WrapError(err, "Connecting to NATS")
		}
		h.connection = connection
		return false, nil
	})

	attemptRetryStrategy := boshretry.NewBackoffWithJitterRetryStrategy(
		natsConnectionMaxRetries,
		h.connectRetryInterval,
		h.maxConnectRetryInterval,
		natsRetryable,
		h.logger,
	)
	err = attemptRetryStrategy.Try()
	if err != nil {
		return bosherr.WrapError(err, "Connecting")
	}

	settings := h.settingsService.GetSettings()

	subject := fmt.Sprintf("agent.%s", settings.AgentID)

	h.logger.Info(h.logTag, "Subscribing to %s", subject)

	_, err = h.connection.Subscribe(subject, func(natsMsg *nats.Msg) {
		// Do not lock handler funcs around possible network calls!
		h.handlerFuncsLock.Lock()
		handlerFuncs := h.handlerFuncs
		h.handlerFuncsLock.Unlock()

		for _, handlerFunc := range handlerFuncs {
			h.handleNatsMsg(natsMsg, handlerFunc)
		}
	})
	if err != nil {
		return bosherr.WrapErrorf(err, "Subscribing to %s", subject)
	}

	return nil
}

func (h *natsHandler) RegisterAdditionalFunc(handlerFunc boshhandler.Func) {
	// Currently not locking since RegisterAdditionalFunc
	// is not a primary way of adding handlerFunc.
	h.handlerFuncsLock.Lock()
	h.handlerFuncs = append(h.handlerFuncs, handlerFunc)
	h.handlerFuncsLock.Unlock()
}

func (h *natsHandler) Send(target boshhandler.Target, topic boshhandler.Topic, message interface{}) error {
	bytes, err := json.Marshal(message)
	if err != nil {
		return bosherr.WrapErrorf(err, "Marshalling message (target=%s, topic=%s): %#v", target, topic, message)
	}

	h.logger.Info(h.logTag, "Sending %s message '%s'", target, topic)
	h.logger.DebugWithDetails(h.logTag, "Message Payload", string(bytes))

	settings := h.settingsService.GetSettings()

	subject := fmt.Sprintf("%s.agent.%s.%s", target, topic, settings.AgentID)
	if h.connection != nil {
		return h.connection.Publish(subject, bytes)
	}
	return nil
}

func (h *natsHandler) Stop() {
	if h.connection != nil {
		h.connection.Close()
	}
}

func (h *natsHandler) VerifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	for _, chain := range verifiedChains {
		if len(chain) == 0 {
			continue
		}
		commonName := chain[0].Subject.CommonName
		match, _ := regexp.MatchString("^[a-zA-Z0-9*\\-]*.nats.bosh-internal$", commonName)
		if match {
			return nil
		}
	}
	return errors.New("Server Certificate CommonName does not match *.nats.bosh-internal")
}

func (h *natsHandler) handleNatsMsg(natsMsg *nats.Msg, handlerFunc boshhandler.Func) {
	respBytes, req, err := boshhandler.PerformHandlerWithJSON(
		natsMsg.Data,
		handlerFunc,
		responseMaxLength,
		h.logger,
	)

	if err != nil {
		h.logger.Error(h.logTag, "Running handler: %s", err)
		h.generateCEFLog(natsMsg, 7, err.Error())
		return
	}

	if len(respBytes) > 0 {
		err = h.connection.Publish(req.ReplyTo, respBytes)
		if err != nil {
			h.generateCEFLog(natsMsg, 7, err.Error())
			h.logger.Error(h.logTag, "Publishing to the client: %s", err.Error())
			return
		}
	}

	h.generateCEFLog(natsMsg, 1, "")
}

func (h *natsHandler) runUntilInterrupted() {
	defer h.connection.Close()

	keepRunning := true

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	for keepRunning {
		select {
		case <-c:
			keepRunning = false
		}
	}
}

func (h *natsHandler) getConnectionInfo() (*ConnectionInfo, error) {
	settings := h.settingsService.GetSettings()

	connInfo := new(ConnectionInfo)
	connInfo.Addr = settings.GetMbusURL()
	natsURL, err := url.Parse(connInfo.Addr)
	if err != nil {
		return nil, bosherr.WrapError(err, "Parsing Nats URL")
	}

	hostSplit := strings.Split(natsURL.Host, ":")
	connInfo.IP = hostSplit[0]

	connInfo.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}

	caCert := settings.GetMbusCerts().CA
	if caCert != "" {
		connInfo.TLSConfig.RootCAs = x509.NewCertPool()
		if ok := connInfo.TLSConfig.RootCAs.AppendCertsFromPEM([]byte(caCert)); !ok {
			return nil, bosherr.Error("Failed to load Mbus CA cert")
		}
	}

	connInfo.TLSConfig.VerifyPeerCertificate = h.VerifyPeerCertificate

	clientCertificate, err := tls.X509KeyPair([]byte(settings.GetMbusCerts().Certificate), []byte(settings.GetMbusCerts().PrivateKey))
	if err != nil {
		return nil, bosherr.WrapError(err, "Parsing certificate and private key")
	}
	connInfo.TLSConfig.Certificates = []tls.Certificate{clientCertificate}

	return connInfo, nil
}

func (h *natsHandler) generateCEFLog(natsMsg *nats.Msg, severity int, statusReason string) {
	cef := boshhandler.NewCommonEventFormat()

	settings := h.settingsService.GetSettings()

	natsURL, err := url.Parse(settings.GetMbusURL())
	if err != nil {
		h.logger.Error(natsHandlerLogTag, err.Error())
		return
	}

	hostSplit := strings.Split(natsURL.Host, ":")
	ip := hostSplit[0]
	payload := struct {
		Method  string `json:"method"`
		ReplyTo string `json:"reply_to"`
	}{}
	err = json.Unmarshal(natsMsg.Data, &payload)
	if err != nil {
		h.logger.Error(natsHandlerLogTag, err.Error())
	}
	cefString, err := cef.ProduceNATSRequestEventLog(ip, hostSplit[1], payload.ReplyTo, payload.Method, severity, natsMsg.Subject, statusReason)

	if err != nil {
		h.logger.Error(natsHandlerLogTag, err.Error())
		return
	}

	if severity == 7 {
		h.auditLogger.Err(cefString)
		return
	}

	h.auditLogger.Debug(cefString)
}
