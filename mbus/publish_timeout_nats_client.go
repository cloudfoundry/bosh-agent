package mbus

import (
	"github.com/cloudfoundry/yagnats"
	"github.com/pivotal-golang/clock"
	"time"
)

var _ yagnats.NATSClient = &PublishTimeoutNatsClient{}

type PublishTimeoutNatsClient struct {
	client yagnats.NATSClient
	clock  clock.Clock
}

func NewPublishTimeoutNatsClient(client yagnats.NATSClient, clock clock.Clock) *PublishTimeoutNatsClient {
	return &PublishTimeoutNatsClient{
		client: client,
		clock:  clock,
	}
}

func (c *PublishTimeoutNatsClient) Ping() bool {
	return c.client.Ping()
}

func (c *PublishTimeoutNatsClient) Connect(connectionProvider yagnats.ConnectionProvider) error {
	return c.client.Connect(connectionProvider)
}

func (c *PublishTimeoutNatsClient) Disconnect() {
	c.client.Disconnect()
}

func (c *PublishTimeoutNatsClient) Publish(subject string, payload []byte) error {
	complete := make(chan error)
	go func() {
		complete <- c.client.Publish(subject, payload)
	}()

	timeout := c.clock.NewTimer(5 * time.Minute)

	select {
	case err := <-complete:
		return err
	case <-timeout.C():
		panic("Publish call to NATSClient took too long, exiting so connections are reset")
	}
}

func (c *PublishTimeoutNatsClient) PublishWithReplyTo(subject, reply string, payload []byte) error {
	panic("not implemented")
}

func (c *PublishTimeoutNatsClient) Subscribe(subject string, callback yagnats.Callback) (int64, error) {
	return c.client.Subscribe(subject, callback)
}

func (c *PublishTimeoutNatsClient) SubscribeWithQueue(subject, queue string, callback yagnats.Callback) (int64, error) {
	return c.client.SubscribeWithQueue(subject, queue, callback)
}

func (c *PublishTimeoutNatsClient) Unsubscribe(subscription int64) error {
	return c.client.Unsubscribe(subscription)
}

func (c *PublishTimeoutNatsClient) UnsubscribeAll(subject string) {
	c.client.UnsubscribeAll(subject)
}

func (c *PublishTimeoutNatsClient) BeforeConnectCallback(callback func()) {
	c.client.BeforeConnectCallback(callback)
}
