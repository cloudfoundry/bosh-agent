package mbus

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudfoundry/yagnats"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

const (
	responseMaxLength = 1024 * 1024
)

type Handler interface {
	Run(boshhandler.Func) error
	Start(boshhandler.Func) error
	RegisterAdditionalFunc(boshhandler.Func)
	Send(target boshhandler.Target, topic boshhandler.Topic, message interface{}) error
	Stop()
}

type natsHandler struct {
	settingsService boshsettings.Service
	client          yagnats.NATSClient
	logger          boshlog.Logger
	handlerFuncs    []boshhandler.Func
	logTag          string
}

func NewNatsHandler(
	settingsService boshsettings.Service,
	client yagnats.NATSClient,
	logger boshlog.Logger,
) Handler {
	return &natsHandler{
		settingsService: settingsService,
		client:          client,
		logger:          logger,
		logTag:          "NATS Handler",
	}
}

func (h *natsHandler) Run(handlerFunc boshhandler.Func) error {
	err := h.Start(handlerFunc)
	if err != nil {
		return bosherr.WrapError(err, "Starting nats handler")
	}

	defer h.Stop()

	h.runUntilInterrupted()

	return nil
}

func (h *natsHandler) Start(handlerFunc boshhandler.Func) error {
	h.RegisterAdditionalFunc(handlerFunc)

	connProvider, err := h.getConnectionInfo()
	if err != nil {
		return bosherr.WrapError(err, "Getting connection info")
	}

	err = h.client.Connect(connProvider)
	if err != nil {
		return bosherr.WrapError(err, "Connecting")
	}

	settings := h.settingsService.GetSettings()

	subject := fmt.Sprintf("agent.%s", settings.AgentID)

	h.logger.Info(h.logTag, "Subscribing to %s", subject)

	_, err = h.client.Subscribe(subject, func(natsMsg *yagnats.Message) {
		for _, handlerFunc := range h.handlerFuncs {
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
	h.handlerFuncs = append(h.handlerFuncs, handlerFunc)
}

func (h natsHandler) Send(target boshhandler.Target, topic boshhandler.Topic, message interface{}) error {
	bytes, err := json.Marshal(message)
	if err != nil {
		return bosherr.WrapErrorf(err, "Marshalling message (target=%s, topic=%s): %#v", target, topic, message)
	}

	h.logger.Info(h.logTag, "Sending %s message '%s'", target, topic)
	h.logger.DebugWithDetails(h.logTag, "Message Payload", string(bytes))

	settings := h.settingsService.GetSettings()

	subject := fmt.Sprintf("%s.agent.%s.%s", target, topic, settings.AgentID)
	return h.client.Publish(subject, bytes)
}

func (h natsHandler) Stop() {
	h.client.Disconnect()
}

func (h natsHandler) handleNatsMsg(natsMsg *yagnats.Message, handlerFunc boshhandler.Func) {
	respBytes, req, err := boshhandler.PerformHandlerWithJSON(
		natsMsg.Payload,
		handlerFunc,
		responseMaxLength,
		h.logger,
	)
	if err != nil {
		h.logger.Error(h.logTag, "Running handler: %s", err)
		return
	}

	if len(respBytes) > 0 {
		h.client.Publish(req.ReplyTo, respBytes)
	}
}

func (h natsHandler) runUntilInterrupted() {
	defer h.client.Disconnect()

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

func (h natsHandler) getConnectionInfo() (*yagnats.ConnectionInfo, error) {
	settings := h.settingsService.GetSettings()

	natsURL, err := url.Parse(settings.Mbus)
	if err != nil {
		return nil, bosherr.WrapError(err, "Parsing Nats URL")
	}

	connInfo := new(yagnats.ConnectionInfo)
	connInfo.Addr = natsURL.Host

	user := natsURL.User
	if user != nil {
		password, passwordIsSet := user.Password()
		if !passwordIsSet {
			return nil, errors.New("No password set for connection")
		}
		connInfo.Password = password
		connInfo.Username = user.Username()
	}

	return connInfo, nil
}
