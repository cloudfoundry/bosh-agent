package mbus_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"

	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	. "github.com/cloudfoundry/bosh-agent/mbus"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

func init() {
	Describe("natsHandler", func() {
		var (
			settingsService *fakesettings.FakeSettingsService
			client          *fakeyagnats.FakeYagnats
			logger          boshlog.Logger
			handler         boshhandler.Handler
		)

		BeforeEach(func() {
			settingsService = &fakesettings.FakeSettingsService{
				Settings: boshsettings.Settings{
					AgentID: "my-agent-id",
					Mbus:    "nats://fake-username:fake-password@127.0.0.1:1234",
				},
			}
			logger = boshlog.NewLogger(boshlog.LevelNone)
			client = fakeyagnats.New()
			handler = NewNatsHandler(settingsService, client, logger)
		})

		Describe("Start", func() {
			It("starts", func() {
				var receivedRequest boshhandler.Request

				handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					receivedRequest = req
					return boshhandler.NewValueResponse("expected value")
				})
				defer handler.Stop()

				Expect(client.ConnectedConnectionProvider()).ToNot(BeNil())

				Expect(client.SubscriptionCount()).To(Equal(1))
				subscriptions := client.Subscriptions("agent.my-agent-id")
				Expect(len(subscriptions)).To(Equal(1))

				expectedPayload := []byte(`{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`)
				subscription := subscriptions[0]
				subscription.Callback(&yagnats.Message{
					Subject: "agent.my-agent-id",
					Payload: expectedPayload,
				})

				Expect(receivedRequest).To(Equal(boshhandler.Request{
					ReplyTo: "reply to me!",
					Method:  "ping",
					Payload: expectedPayload,
				}))

				Expect(client.PublishedMessageCount()).To(Equal(1))
				messages := client.PublishedMessages("reply to me!")
				Expect(len(messages)).To(Equal(1))
				Expect(messages[0].Payload).To(Equal([]byte(`{"value":"expected value"}`)))
			})

			It("does not respond if the response is nil", func() {
				err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					return nil
				})
				Expect(err).ToNot(HaveOccurred())
				defer handler.Stop()

				subscription := client.Subscriptions("agent.my-agent-id")[0]
				subscription.Callback(&yagnats.Message{
					Subject: "agent.my-agent-id",
					Payload: []byte(`{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`),
				})

				Expect(client.PublishedMessageCount()).To(Equal(0))
			})

			It("does not respond if the request sender is omitted", func() {
				err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					return boshhandler.NewValueResponse("pong")
				})
				Expect(err).ToNot(HaveOccurred())
				defer handler.Stop()

				subscription := client.Subscriptions("agent.my-agent-id")[0]
				subscription.Callback(&yagnats.Message{
					Subject: "agent.my-agent-id",
					Payload: []byte(`{"method":"ping","arguments":["foo","bar"]}`),
				})

				Expect(client.PublishedMessageCount()).To(Equal(0))
			})

			It("responds with an error if the response is bigger than 1MB", func() {
				err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					// gets inflated by json.Marshal when enveloping
					size := 0

					switch req.Method {
					case "small":
						size = 1024*1024 - 12
					case "big":
						size = 1024 * 1024
					default:
						panic("unknown request size")
					}

					chars := make([]byte, size)
					for i := range chars {
						chars[i] = 'A'
					}
					return boshhandler.NewValueResponse(string(chars))
				})
				Expect(err).ToNot(HaveOccurred())
				defer handler.Stop()

				subscription := client.Subscriptions("agent.my-agent-id")[0]
				subscription.Callback(&yagnats.Message{
					Subject: "agent.my-agent-id",
					Payload: []byte(`{"method":"small","arguments":[], "reply_to": "fake-reply-to"}`),
				})

				subscription.Callback(&yagnats.Message{
					Subject: "agent.my-agent-id",
					Payload: []byte(`{"method":"big","arguments":[], "reply_to": "fake-reply-to"}`),
				})

				Expect(client.PublishedMessageCount()).To(Equal(1))
				messages := client.PublishedMessages("fake-reply-to")
				Expect(len(messages)).To(Equal(2))
				Expect(messages[0].Payload).To(MatchRegexp("value"))
				Expect(messages[1].Payload).To(Equal([]byte(
					`{"exception":{"message":"Response exceeded maximum allowed length"}}`)))
			})

			It("can add additional handler funcs to receive requests", func() {
				var firstHandlerReq, secondHandlerRequest boshhandler.Request

				handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					firstHandlerReq = req
					return boshhandler.NewValueResponse("first-handler-resp")
				})
				defer handler.Stop()

				handler.RegisterAdditionalFunc(func(req boshhandler.Request) (resp boshhandler.Response) {
					secondHandlerRequest = req
					return boshhandler.NewValueResponse("second-handler-resp")
				})

				expectedPayload := []byte(`{"method":"ping","arguments":["foo","bar"], "reply_to": "fake-reply-to"}`)

				subscription := client.Subscriptions("agent.my-agent-id")[0]
				subscription.Callback(&yagnats.Message{
					Subject: "agent.my-agent-id",
					Payload: expectedPayload,
				})

				// Expected requests received by both handlers
				Expect(firstHandlerReq).To(Equal(boshhandler.Request{
					ReplyTo: "fake-reply-to",
					Method:  "ping",
					Payload: expectedPayload,
				}))

				Expect(secondHandlerRequest).To(Equal(boshhandler.Request{
					ReplyTo: "fake-reply-to",
					Method:  "ping",
					Payload: expectedPayload,
				}))

				// Bosh handler responses were sent
				Expect(client.PublishedMessageCount()).To(Equal(1))
				messages := client.PublishedMessages("fake-reply-to")
				Expect(len(messages)).To(Equal(2))
				Expect(messages[0].Payload).To(Equal([]byte(`{"value":"first-handler-resp"}`)))
				Expect(messages[1].Payload).To(Equal([]byte(`{"value":"second-handler-resp"}`)))
			})

			It("has the correct connection info", func() {
				err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
				Expect(err).ToNot(HaveOccurred())
				defer handler.Stop()

				Expect(client.ConnectedConnectionProvider()).To(Equal(&yagnats.ConnectionInfo{
					Addr:     "127.0.0.1:1234",
					Username: "fake-username",
					Password: "fake-password",
				}))
			})

			It("does not err when no username and password", func() {
				settingsService.Settings.Mbus = "nats://127.0.0.1:1234"
				handler = NewNatsHandler(settingsService, client, logger)

				err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
				Expect(err).ToNot(HaveOccurred())
				defer handler.Stop()
			})

			It("errs when has username without password", func() {
				settingsService.Settings.Mbus = "nats://foo@127.0.0.1:1234"
				handler = NewNatsHandler(settingsService, client, logger)

				err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
				Expect(err).To(HaveOccurred())
				defer handler.Stop()
			})
		})

		Describe("Send", func() {
			It("sends the message over nats to a subject that includes the target and topic", func() {
				errCh := make(chan error, 1)

				payload := map[string]string{"key1": "value1", "keyA": "valueA"}

				go func() {
					errCh <- handler.Send(boshhandler.HealthMonitor, boshhandler.Heartbeat, payload)
				}()

				var err error
				select {
				case err = <-errCh:
				}
				Expect(err).ToNot(HaveOccurred())

				Expect(client.PublishedMessageCount()).To(Equal(1))
				messages := client.PublishedMessages("hm.agent.heartbeat.my-agent-id")
				Expect(messages).To(HaveLen(1))
				Expect(messages[0].Payload).To(Equal(
					[]byte("{\"key1\":\"value1\",\"keyA\":\"valueA\"}"),
				))
			})
		})
	})
}
