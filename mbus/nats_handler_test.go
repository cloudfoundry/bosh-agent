package mbus_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"

	"github.com/nats-io/nats.go"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	boshhandler "github.com/cloudfoundry/bosh-agent/v2/handler"
	"github.com/cloudfoundry/bosh-agent/v2/mbus"
	"github.com/cloudfoundry/bosh-agent/v2/mbus/mbusfakes"
	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/v2/settings/fakes"
)

func init() { //nolint:funlen,gochecknoinits
	Describe("natsHandler", func() {
		var (
			settingsService     *fakesettings.FakeSettingsService
			connector           mbus.NatsConnector
			connectorURLArg     string
			connectorOptionsArg []nats.Option
			connection          *mbusfakes.FakeNatsConnection
			logger              boshlog.Logger
			handler             boshhandler.Handler
			platform            *platformfakes.FakePlatform
			auditLogger         *platformfakes.FakeAuditLogger
			loggerOutBuf        *bytes.Buffer
		)
		ValidCA, _ := os.ReadFile("./test_assets/ca.pem")                   //nolint:errcheck
		ValidCertificate, _ := os.ReadFile("./test_assets/client-cert.pem") //nolint:errcheck
		ValidPrivateKey, _ := os.ReadFile("./test_assets/client-pkey.pem")  //nolint:errcheck

		BeforeEach(func() {
			settingsService = &fakesettings.FakeSettingsService{
				Settings: boshsettings.Settings{
					AgentID: "my-agent-id",
					Mbus:    "nats://fake-username:fake-password@127.0.0.1:1234",
					Env: boshsettings.Env{
						Bosh: boshsettings.BoshEnv{
							Mbus: boshsettings.MBus{
								Cert: boshsettings.CertKeyPair{
									CA:          string(ValidCA),
									PrivateKey:  string(ValidPrivateKey),
									Certificate: string(ValidCertificate),
								},
							},
						},
					},
				},
			}

			loggerOutBuf = bytes.NewBufferString("")
			logger = boshlog.NewWriterLogger(boshlog.LevelError, loggerOutBuf)
			connection = &mbusfakes.FakeNatsConnection{}

			connector = func(url string, options ...nats.Option) (mbus.NatsConnection, error) {
				connectorURLArg = url
				connectorOptionsArg = options
				return connection, nil
			}
			platform = &platformfakes.FakePlatform{}
			auditLogger = &platformfakes.FakeAuditLogger{}
			platform.GetAuditLoggerReturns(auditLogger)
		})

		JustBeforeEach(func() {
			handler = mbus.NewNatsHandler(settingsService, connector, logger, platform)
		})

		Describe("Start", func() {
			It("starts", func() {
				var receivedRequest boshhandler.Request

				err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					receivedRequest = req
					return boshhandler.NewValueResponse("expected value")
				})
				Expect(err).NotTo(HaveOccurred())
				defer handler.Stop()

				Expect(connection.SubscribeCallCount()).To(Equal(1))
				subj, handler := connection.SubscribeArgsForCall(0)
				Expect(subj).To(Equal("agent.my-agent-id"))

				expectedPayload := []byte(`{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`)
				handler(&nats.Msg{
					Subject: "agent.my-agent-id",
					Data:    expectedPayload,
				})

				Expect(receivedRequest).To(Equal(boshhandler.Request{
					ReplyTo: "reply to me!",
					Method:  "ping",
					Payload: expectedPayload,
				}))

				Expect(connection.PublishCallCount()).To(Equal(1))
				subj, message := connection.PublishArgsForCall(0)
				Expect(subj).To(Equal("reply to me!"))
				Expect(message).To(Equal([]byte(`{"value":"expected value"}`)))
			})

			It("cleans up ip-mac address cache for nats configured with ip address", func() {
				err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					return nil
				})
				Expect(err).NotTo(HaveOccurred())
				defer handler.Stop()

				Expect(platform.DeleteARPEntryWithIPArgsForCall(0)).To(Equal("127.0.0.1"))
			})

			It("does not try to clean up ip-mac address cache for nats configured with hostname", func() {
				settingsService.Settings.Mbus = "nats://fake-username:fake-password@fake-hostname.com:1234"
				err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					return nil
				})
				Expect(err).NotTo(HaveOccurred())
				defer handler.Stop()

				Expect(platform.DeleteARPEntryWithIPCallCount()).To(Equal(0))
			})

			It("logs error and proceeds if it fails to clean up ip-mac address cache for nats", func() {
				platform.DeleteARPEntryWithIPReturns(errors.New("failed to run"))
				err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					return nil
				})
				Expect(err).NotTo(HaveOccurred())
				defer handler.Stop()

				Expect(platform.DeleteARPEntryWithIPArgsForCall(0)).To(Equal("127.0.0.1"))
				Expect(loggerOutBuf).To(ContainSubstring("ERROR - Cleaning ip-mac address cache for: 127.0.0.1"))
			})

			It("does not respond if the response is nil", func() {
				err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					return nil
				})
				Expect(err).ToNot(HaveOccurred())
				defer handler.Stop()

				_, handler := connection.SubscribeArgsForCall(0)
				handler(&nats.Msg{
					Subject: "agent.my-agent-id",
					Data:    []byte(`{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`),
				})

				Expect(connection.PublishCallCount()).To(Equal(0))
			})

			It("responds with an error if the response is bigger than 1MB", func() {
				err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					chars := make([]byte, 1024*1024)
					for i := range chars {
						chars[i] = 'A'
					}
					return boshhandler.NewValueResponse(string(chars))
				})
				Expect(err).ToNot(HaveOccurred())
				defer handler.Stop()

				_, handler := connection.SubscribeArgsForCall(0)
				handler(&nats.Msg{
					Subject: "agent.my-agent-id",
					Data:    []byte(`{"method":"big","arguments":[], "reply_to": "fake-reply-to"}`),
				})

				Expect(connection.PublishCallCount()).To(Equal(1))
				subj, message := connection.PublishArgsForCall(0)
				Expect(subj).To(Equal("fake-reply-to"))
				Expect(message).To(Equal([]byte(
					`{"exception":{"message":"Response exceeded maximum allowed length"}}`)))
			})

			It("can add additional handler funcs to receive requests", func() {
				var firstHandlerReq, secondHandlerRequest boshhandler.Request

				err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					firstHandlerReq = req
					return boshhandler.NewValueResponse("first-handler-resp")
				})
				Expect(err).NotTo(HaveOccurred())
				defer handler.Stop()

				handler.RegisterAdditionalFunc(func(req boshhandler.Request) (resp boshhandler.Response) {
					secondHandlerRequest = req
					return boshhandler.NewValueResponse("second-handler-resp")
				})

				expectedPayload := []byte(`{"method":"ping","arguments":["foo","bar"], "reply_to": "fake-reply-to"}`)

				_, handler := connection.SubscribeArgsForCall(0)
				handler(&nats.Msg{
					Subject: "agent.my-agent-id",
					Data:    expectedPayload,
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
				Expect(connection.PublishCallCount()).To(Equal(2))
				subj, message := connection.PublishArgsForCall(0)
				Expect(subj).To(Equal("fake-reply-to"))
				Expect(message).To(Equal([]byte(`{"value":"first-handler-resp"}`)))
				subj, message = connection.PublishArgsForCall(1)
				Expect(subj).To(Equal("fake-reply-to"))
				Expect(message).To(Equal([]byte(`{"value":"second-handler-resp"}`)))
			})

			It("has the correct connection info", func() {
				err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
				Expect(err).ToNot(HaveOccurred())
				defer handler.Stop()

				Expect(connectorURLArg).To(Equal("nats://fake-username:fake-password@127.0.0.1:1234"))
			})

			It("does not err when no username and password", func() {
				settingsService.Settings.Mbus = "nats://127.0.0.1:1234"
				handler = mbus.NewNatsHandler(settingsService, connector, logger, platform)

				err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
				Expect(err).ToNot(HaveOccurred())
				defer handler.Stop()
			})

			Context("CEF logging", func() {
				It("logs to syslog debug", func() {
					err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
						return nil
					})
					Expect(err).ToNot(HaveOccurred())
					defer handler.Stop()

					_, handler := connection.SubscribeArgsForCall(0)
					handler(&nats.Msg{
						Subject: "agent.my-agent-id",
						Data:    []byte(`{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`),
					})

					msg := `CEF:0|CloudFoundry|BOSH|1|agent_api|ping|1|duser=reply to me! src=127.0.0.1 spt=1234`
					Expect(auditLogger.DebugArgsForCall(0)).To(ContainSubstring(msg))
				})

				Context("when NATs handler has an error", func() {
					It("logs to syslog error", func() {
						err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
							return nil
						})
						Expect(err).ToNot(HaveOccurred())
						defer handler.Stop()

						_, handler := connection.SubscribeArgsForCall(0)
						handler(&nats.Msg{
							Subject: "agent.my-agent-id",
							Data:    []byte(`bad json`),
						})

						Expect(auditLogger.DebugCallCount()).To(Equal(0))
						Expect(auditLogger.ErrCallCount()).To(Equal(1))
						msg := `cs1=Unmarshalling JSON payload: invalid character 'b' looking for beginning of value cs1Label=statusReason`
						Expect(auditLogger.ErrArgsForCall(0)).To(ContainSubstring(msg))
					})
				})

				Context("when NATs handler fails to publish", func() {
					It("logs to syslog error", func() {
						connection.PublishReturns(errors.New("oh noes"))

						err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
							return boshhandler.NewValueResponse("responding")
						})
						Expect(err).ToNot(HaveOccurred())
						defer handler.Stop()

						_, handler := connection.SubscribeArgsForCall(0)
						handler(&nats.Msg{
							Subject: "agent.my-agent-id",
							Data:    []byte(`{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`),
						})

						Expect(auditLogger.DebugCallCount()).To(Equal(0))
						Expect(auditLogger.ErrCallCount()).To(Equal(1))
						msg := `cs1=oh noes cs1Label=statusReason`
						Expect(auditLogger.ErrArgsForCall(0)).To(ContainSubstring(msg))
					})
				})
			})

			Context("Mutual TLS", func() {
				It("sets Client Certificates and Server CA on the TLSConfig", func() {
					err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
					Expect(err).ToNot(HaveOccurred())
					defer handler.Stop()

					certPool := x509.NewCertPool()
					ok := certPool.AppendCertsFromPEM(ValidCA)
					Expect(ok).To(BeTrue())

					clientCert, err := tls.LoadX509KeyPair("./test_assets/client-cert.pem", "./test_assets/client-pkey.pem")
					Expect(err).ToNot(HaveOccurred())

					options := nats.Options{}
					for _, option := range connectorOptionsArg {
						err := option(&options)
						Expect(err).NotTo(HaveOccurred())
					}

					Expect(options.TLSConfig.RootCAs.Subjects()).To(BeEquivalentTo(certPool.Subjects())) //nolint:staticcheck
					Expect(options.TLSConfig.Certificates[0]).To(Equal(clientCert))
				})

				It("returns an error if the `ca cert` is provided and invalid", func() {
					settingsService.Settings.Env.Bosh.Mbus.Cert.CA = "Invalid Cert"

					err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Getting connection info: Failed to load Mbus CA cert"))
				})

				It("returns an error if the client certificate is invalid", func() {
					settingsService.Settings.Env.Bosh.Mbus.Cert.Certificate = "Invalid Client Certificate"

					err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Getting connection info: Parsing certificate and private key: tls: failed to find any PEM data in certificate input"))
				})

				It("returns an error if the private key is invalid", func() {
					settingsService.Settings.Env.Bosh.Mbus.Cert.PrivateKey = "Invalid Private Key"

					err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Getting connection info: Parsing certificate and private key: tls: failed to find any PEM data in key input"))
				})

				Context("when the VerifyPeerCertificate is called", func() {
					It("verify certificate common name matches correct pattern", func() {
						certPath := "test_assets/custom_cert.pem"
						caPath := "test_assets/ca.pem"
						err := VerifyPeerCertificateCallback(handler, connectorOptionsArg, certPath, caPath)

						Expect(err).To(BeNil())
					})

					It("verify certificate common name does not match the correct pattern", func() {
						certPath := "test_assets/invalid_cn_cert.pem"
						caPath := "test_assets/ca.pem"
						err := VerifyPeerCertificateCallback(handler, connectorOptionsArg, certPath, caPath)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("server Certificate CommonName does not match *.nats.bosh-internal"))
					})

					It("verify certificate common name is missing", func() {
						certPath := "test_assets/missing_cn_cert.pem"
						caPath := "test_assets/ca.pem"
						err := VerifyPeerCertificateCallback(handler, connectorOptionsArg, certPath, caPath)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("server Certificate CommonName does not match *.nats.bosh-internal"))
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

						options := nats.Options{}
						for _, option := range connectorOptionsArg {
							err := option(&options)
							Expect(err).NotTo(HaveOccurred())
						}

						Expect(options.TLSConfig.RootCAs).To(BeNil())
						Expect(options.TLSConfig.Certificates[0]).To(Equal(clientCert))
					})
				})
			})
		})

		Describe("Send", func() {
			It("sends the message over nats to a subject that includes the target and topic", func() {
				err := handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
					return nil
				})
				Expect(err).ToNot(HaveOccurred())
				defer handler.Stop()

				payload := map[string]string{"key1": "value1", "keyA": "valueA"}

				err = handler.Send(boshhandler.HealthMonitor, boshhandler.Heartbeat, payload)
				Expect(err).ToNot(HaveOccurred())

				Expect(connection.PublishCallCount()).To(Equal(1))
				subj, message := connection.PublishArgsForCall(0)
				Expect(subj).To(Equal("hm.agent.heartbeat.my-agent-id"))
				Expect(message).To(Equal([]byte("{\"key1\":\"value1\",\"keyA\":\"valueA\"}")))
			})
		})
	})
}

func VerifyPeerCertificateCallback(handler boshhandler.Handler, connectorOptionsArg []nats.Option, certPath string, caPath string) error {
	ValidCA, _ := os.ReadFile("./test_assets/ca.pem") //nolint:errcheck

	correctCnCert, err := os.ReadFile(certPath)
	Expect(err).NotTo(HaveOccurred())
	correctCa, err := os.ReadFile(caPath)
	Expect(err).NotTo(HaveOccurred())

	certPemBlock, _ := pem.Decode(correctCnCert)
	cert, err := x509.ParseCertificate(certPemBlock.Bytes)
	Expect(err).NotTo(HaveOccurred())
	caPemBlock, _ := pem.Decode(correctCa)
	ca, err := x509.ParseCertificate(caPemBlock.Bytes)
	Expect(err).NotTo(HaveOccurred())

	errHandler := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
	Expect(errHandler).ToNot(HaveOccurred())
	defer handler.Stop()

	options := nats.Options{}
	for _, option := range connectorOptionsArg {
		err := option(&options)
		Expect(err).NotTo(HaveOccurred())
	}

	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(ValidCA)
	Expect(ok).To(BeTrue())

	callback := options.TLSConfig.VerifyPeerCertificate

	raw := [][]byte{correctCnCert, correctCa}
	verified := [][]*x509.Certificate{{cert, ca}}

	err = callback(raw, verified)
	return err
}
