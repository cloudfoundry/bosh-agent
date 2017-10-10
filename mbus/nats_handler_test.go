package mbus_test

import (
	"bytes"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"

	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	. "github.com/cloudfoundry/bosh-agent/mbus"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"io/ioutil"
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
		)

		BeforeEach(func() {
			settingsService = &fakesettings.FakeSettingsService{
				Settings: boshsettings.Settings{
					AgentID: "my-agent-id",
					Mbus:    "nats://fake-username:fake-password@127.0.0.1:1234",
				},
			}

			loggerOutBuf = bytes.NewBufferString("")
			logger = boshlog.NewWriterLogger(boshlog.LevelError, loggerOutBuf)

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
				Expect(loggerOutBuf).To(ContainSubstring("ERROR - Cleaning ip-mac address cache for: 127.0.0.1"))
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

			Context("Mutual TLS", func() {
				ValidCA, _ := ioutil.ReadFile("./test_assets/ca.pem")
				ValidCertificate, _ := ioutil.ReadFile("./test_assets/client-cert.pem")
				ValidPrivateKey, _ := ioutil.ReadFile("./test_assets/client-pkey.pem")

				BeforeEach(func() {
					settingsService.Settings.Env.Bosh.Mbus = boshsettings.MBus{
						Cert: boshsettings.CertKeyPair{
							CA:          string(ValidCA),
							PrivateKey:  string(ValidPrivateKey),
							Certificate: string(ValidCertificate),
						},
						URLs: []string{"tls://fake-username:fake-password@127.0.0.1:1234"},
					}
				})

				It("sets CertPool and ClientCert on ConnectionInfo", func() {
					err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
					Expect(err).ToNot(HaveOccurred())
					defer handler.Stop()

					certPool := x509.NewCertPool()
					ok := certPool.AppendCertsFromPEM(ValidCA)
					Expect(ok).To(BeTrue())

					clientCert, err := tls.LoadX509KeyPair("./test_assets/client-cert.pem", "./test_assets/client-pkey.pem")

					Expect(err, BeNil())

					result := client.ConnectedConnectionProvider().(*yagnats.ConnectionInfo)
					expected := &yagnats.ConnectionInfo{
						Addr:     "127.0.0.1:1234",
						Username: "fake-username",
						Password: "fake-password",
						TLSInfo: &yagnats.ConnectionTLSInfo{
							CertPool:   certPool,
							ClientCert: &clientCert,
						},
					}

					Expect(result.Addr).To(Equal(expected.Addr))
					Expect(result.Username).To(Equal(expected.Username))
					Expect(result.Password).To(Equal(expected.Password))
					Expect(result.TLSInfo.CertPool).To(Equal(expected.TLSInfo.CertPool))
					Expect(result.TLSInfo.ClientCert).To(Equal(expected.TLSInfo.ClientCert))
				})

				It("returns an error if the `ca cert` is provided and invalid", func() {
					settingsService.Settings.Env.Bosh.Mbus.Cert.CA = "Invalid Cert"

					err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Getting connection info: Failed to load Mbus CA cert"))
					defer handler.Stop()
				})

				It("returns an error if the client certificate is invalid", func() {
					settingsService.Settings.Env.Bosh.Mbus.Cert.Certificate = "Invalid Client Certificate"

					err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Getting connection info: Parsing certificate and private key: tls: failed to find any PEM data in certificate input"))
					defer handler.Stop()
				})

				It("returns an error if the private key is invalid", func() {
					settingsService.Settings.Env.Bosh.Mbus.Cert.PrivateKey = "Invalid Private Key"

					err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Getting connection info: Parsing certificate and private key: tls: failed to find any PEM data in key input"))
					defer handler.Stop()
				})

				Context("when the VerifyPeerCertificate is called", func() {
					It("verify certificate common name matches correct pattern", func() {
						certPath := "test_assets/custom_cert.pem"
						caPath := "test_assets/ca.pem"
						err := testVerifyPeerCertificateCallback(client, handler, certPath, caPath)

						Expect(err).To(BeNil())
					})

					It("verify certificate common name does not match the correct pattern", func() {
						certPath := "test_assets/invalid_cn_cert.pem"
						caPath := "test_assets/ca.pem"
						err := testVerifyPeerCertificateCallback(client, handler, certPath, caPath)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("Server Certificate CommonName does not match *.nats.bosh-internal"))
					})

					It("verify certificate common name is missing", func() {
						certPath := "test_assets/missing_cn_cert.pem"
						caPath := "test_assets/ca.pem"
						err := testVerifyPeerCertificateCallback(client, handler, certPath, caPath)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("Server Certificate CommonName does not match *.nats.bosh-internal"))
					})
				})

				Context("when `ca cert` is not passed", func() {
					It("should not try to append blank `ca cert` (should only rely on system trusted certs)", func() {
						settingsService.Settings.Env.Bosh.Mbus.Cert.CA = ""

						err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
						Expect(err).NotTo(HaveOccurred())
						defer handler.Stop()

						clientCert, err := tls.LoadX509KeyPair("./test_assets/client-cert.pem", "./test_assets/client-pkey.pem")
						Expect(err, BeNil())

						result := client.ConnectedConnectionProvider().(*yagnats.ConnectionInfo)
						expected := &yagnats.ConnectionInfo{
							Addr:     "127.0.0.1:1234",
							Username: "fake-username",
							Password: "fake-password",
							TLSInfo: &yagnats.ConnectionTLSInfo{
								ClientCert: &clientCert,
							},
						}

						Expect(result.TLSInfo.CertPool).To(BeNil())
						Expect(result.TLSInfo.ClientCert).To(Equal(expected.TLSInfo.ClientCert))
					})
				})
			})

			Context("when connecting to NATS server fails", func() {
				BeforeEach(func() {
					client.SetConnectErrors([]error{
						errors.New("error"),
						errors.New("error"),
					})
				})

				It("will retry the max number allowed", func() {
					var receivedRequest boshhandler.Request

					err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
						receivedRequest = req
						return boshhandler.NewValueResponse("expected value")
					})
					defer handler.Stop()

					Expect(err).To(BeNil())
					Expect(client.GetConnectCallCount()).To(Equal(3))

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

				Context("when exhausting all the retries", func() {

					BeforeEach(func() {
						client.SetConnectErrors([]error{
							errors.New("Nats Connection Error 1"),
							errors.New("Nats Connection Error 2"),
							errors.New("Nats Connection Error 3"),
							errors.New("Nats Connection Error 4"),
							errors.New("Nats Connection Error 5"),
						})
					})

					It("will return an error", func() {
						var receivedRequest boshhandler.Request

						err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
							receivedRequest = req
							return boshhandler.NewValueResponse("expected value")
						})
						defer handler.Stop()

						Expect(client.GetConnectCallCount()).To(Equal(4))
						Expect(err).ToNot(BeNil())
						Expect(err.Error()).To(ContainSubstring("Nats Connection Error 4"))
					})
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

func testVerifyPeerCertificateCallback(client *fakeyagnats.FakeYagnats, handler boshhandler.Handler, certPath string, caPath string) error {
	ValidCA, _ := ioutil.ReadFile("./test_assets/ca.pem")

	correctCnCert, err := ioutil.ReadFile(certPath)
	Expect(err).NotTo(HaveOccurred())
	correctCa, err := ioutil.ReadFile(caPath)
	Expect(err).NotTo(HaveOccurred())

	certPemBlock, _ := pem.Decode([]byte(correctCnCert))
	cert, err := x509.ParseCertificate(certPemBlock.Bytes)
	caPemBlock, _ := pem.Decode([]byte(correctCa))
	ca, err := x509.ParseCertificate(caPemBlock.Bytes)

	errHandler := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
	Expect(errHandler).ToNot(HaveOccurred())
	defer handler.Stop()

	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(ValidCA)
	Expect(ok).To(BeTrue())

	result := client.ConnectedConnectionProvider().(*yagnats.ConnectionInfo)
	callback := result.TLSInfo.VerifyPeerCertificate

	raw := [][]byte{correctCnCert, correctCa}
	verified := [][]*x509.Certificate{{cert, ca}}

	err = callback(raw, verified)
	return err
}
