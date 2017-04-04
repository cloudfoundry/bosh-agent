package mbus_test

import (
	"bytes"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"

	"crypto/x509"
	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	. "github.com/cloudfoundry/bosh-agent/mbus"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
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
			platform        *fakeplatform.FakePlatform
			loggerOutBuf    *bytes.Buffer
			loggerErrBuf    *bytes.Buffer
		)

		BeforeEach(func() {
			settingsService = &fakesettings.FakeSettingsService{
				Settings: boshsettings.Settings{
					AgentID: "my-agent-id",
					Mbus:    "nats://fake-username:fake-password@127.0.0.1:1234",
				},
			}

			loggerOutBuf = bytes.NewBufferString("")
			loggerErrBuf = bytes.NewBufferString("")
			logger = boshlog.NewWriterLogger(boshlog.LevelError, loggerOutBuf, loggerErrBuf)

			client = fakeyagnats.New()
			platform = fakeplatform.NewFakePlatform()
			handler = NewNatsHandler(settingsService, client, logger, platform)
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

			It("cleans up ip-mac address cache for nats configured with ip address", func() {
				handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					return nil
				})
				defer handler.Stop()

				Expect(platform.LastIPDeletedFromARP).To(Equal("127.0.0.1"))
				Expect(client.ConnectedConnectionProvider()).ToNot(BeNil())
			})

			It("does not try to clean up ip-mac address cache for nats configured with hostname", func() {
				settingsService.Settings.Mbus = "nats://fake-username:fake-password@fake-hostname.com:1234"
				handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					return nil
				})
				defer handler.Stop()

				Expect(platform.LastIPDeletedFromARP).To(BeEmpty())
				Expect(client.ConnectedConnectionProvider()).ToNot(BeNil())
			})

			It("logs error and proceeds if it fails to clean up ip-mac address cache for nats", func() {
				platform.DeleteARPEntryWithIPErr = errors.New("failed to run")
				handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					return nil
				})
				defer handler.Stop()

				Expect(platform.LastIPDeletedFromARP).To(Equal("127.0.0.1"))
				Expect(loggerErrBuf).To(ContainSubstring("ERROR - Cleaning ip-mac address cache for: 127.0.0.1"))
				Expect(client.ConnectedConnectionProvider()).ToNot(BeNil())
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
				handler = NewNatsHandler(settingsService, client, logger, platform)

				err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
				Expect(err).ToNot(HaveOccurred())
				defer handler.Stop()
			})

			It("errs when has username without password", func() {
				settingsService.Settings.Mbus = "nats://foo@127.0.0.1:1234"
				handler = NewNatsHandler(settingsService, client, logger, platform)

				err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
				Expect(err).To(HaveOccurred())
				defer handler.Stop()
			})

			Context("CEF logging", func() {
				It("logs to syslog debug", func() {
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

					auditLogger := platform.GetAuditLogger().(*fakeplatform.FakeAuditLogger)

					Expect(auditLogger.GetDebugMsgs()[0]).To(ContainSubstring("CEF:0|CloudFoundry|BOSH|1|agent_api|ping|1|duser=reply to me! src=127.0.0.1 spt=1234"))
				})

				Context("when NATs handler has an error", func() {
					It("logs to syslog error", func() {
						err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
							return nil
						})
						Expect(err).ToNot(HaveOccurred())
						defer handler.Stop()

						subscription := client.Subscriptions("agent.my-agent-id")[0]
						subscription.Callback(&yagnats.Message{
							Subject: "agent.my-agent-id",
							Payload: []byte(`bad json`),
						})

						auditLogger := platform.GetAuditLogger().(*fakeplatform.FakeAuditLogger)

						Expect(auditLogger.GetDebugMsgs()).To(BeEmpty())
						Expect(auditLogger.GetErrMsgs()[0]).To(ContainSubstring(`cs1=Unmarshalling JSON payload: invalid character 'b' looking for beginning of value cs1Label=statusReason`))
					})
				})

				Context("when NATs handler fails to publish", func() {
					It("logs to syslog error", func() {
						client.WhenPublishing("reply to me!", func(*yagnats.Message) error {
							return errors.New("Oh noes!")
						})

						err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
							return boshhandler.NewValueResponse("responding")
						})
						Expect(err).ToNot(HaveOccurred())
						defer handler.Stop()

						subscription := client.Subscriptions("agent.my-agent-id")[0]
						subscription.Callback(&yagnats.Message{
							Subject: "agent.my-agent-id",
							Payload: []byte(`{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`),
						})

						auditLogger := platform.GetAuditLogger().(*fakeplatform.FakeAuditLogger)

						Expect(auditLogger.GetDebugMsgs()).To(BeEmpty())
						Expect(auditLogger.GetErrMsgs()[0]).To(ContainSubstring(`cs1=Oh noes! cs1Label=statusReason`))
					})
				})
			})

			Context("TLS", func() {
				var CA = `-----BEGIN CERTIFICATE-----
MIIDFDCCAfygAwIBAgIRANn247vhGXLev3Ltw8NOIQAwDQYJKoZIhvcNAQELBQAw
MzEMMAoGA1UEBhMDVVNBMRYwFAYDVQQKEw1DbG91ZCBGb3VuZHJ5MQswCQYDVQQD
EwJjYTAeFw0xNzA0MDMxOTQyMTVaFw0xODA0MDMxOTQyMTVaMDMxDDAKBgNVBAYT
A1VTQTEWMBQGA1UEChMNQ2xvdWQgRm91bmRyeTELMAkGA1UEAxMCY2EwggEiMA0G
CSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCpcve+8iQmzj2/dUfGpI55CmZ7aYAr
CsXsB6ztceewhMKTOo1pgBYT5T3G759Leab4id2JjxJB2sjou7g69pCTWNFkKz0G
ED2RMGfuMACfISezE5fhSKdNR0vyleSEgvwOcdWa0PP6pTK//iD7p4fyx5HigpWt
7hxmUTsqzOBOOYv1tw7ZhX6msZ5EL4d58rIbqozz8Hr/5mw/izUr2w0dCuuXTb8k
qIrh1PjPwBoOW38yXZ/Pyex14NQMiqVqH2gMSwXpZNdVi9whVGrzP3ZAUv5uyICK
j4KGBFJ+NcFq9VI2lbBUNdCD4MqdzaSA7OSnhaYYku2KUwIlBG9CtQctAgMBAAGj
IzAhMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEB
CwUAA4IBAQBLcq6xTPGYhA5Blhbkja3kd7AsWsVOv/HvLnxUJLwY8SLDhDHVndRR
NvNmmXQOMDZ9tcLUl4Jgoy+u2XnQxTfpvPwT0qX958spcwCo9mQJKuOFcZfNwS8M
bSTo1k+a33YtB8AWyS0GabG+2PEp/ARptJiQ6OMDKDLFMKK4NqpSl8cXNmPf5bEO
67qHgr+2xtS4Mkj+EhZJuVpqIU3jL7psIQWdEm7dAy+qmZaB44LT1AMcUINgBsor
bew6/PW7wNhEW/GWI/Nvef3EsFh80bYHq21eW6RdaSLgwddcmi6ak4CxizPYK57e
XtrIuun84K30EXBrBdtUqWBwgBtu/HT2
-----END CERTIFICATE-----`
				BeforeEach(func() {
					settingsService.Settings.Env.Bosh.Mbus = &boshsettings.MBus{
						URL: "tls://fake-username:fake-password@127.0.0.1:1234",
						CA:  CA,
					}
				})

				It("adds Cert pool with an entry to ConnectionInfo.CertPool", func() {
					err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
					Expect(err).ToNot(HaveOccurred())
					defer handler.Stop()

					certPool := x509.NewCertPool()
					ok := certPool.AppendCertsFromPEM([]byte(CA))
					Expect(ok).To(BeTrue())

					Expect(client.ConnectedConnectionProvider()).To(Equal(&yagnats.ConnectionInfo{
						Addr:     "127.0.0.1:1234",
						Username: "fake-username",
						Password: "fake-password",
						CertPool: certPool,
					}))
				})

				It("returns an error if the cert is empty", func() {
					settingsService.Settings.Env.Bosh.Mbus.CA = ""

					err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Getting connection info: Failed to load Mbus CA cert"))
					defer handler.Stop()
				})

				It("returns an error if the cert is invalid", func() {
					settingsService.Settings.Env.Bosh.Mbus.CA = "Invalid Cert"

					err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Getting connection info: Failed to load Mbus CA cert"))
					defer handler.Stop()
				})
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
