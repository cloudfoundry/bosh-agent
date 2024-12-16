package mbus_test

import (
	"net/url"
	"reflect"

	"github.com/nats-io/nats.go"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/agent/blobstore/blobstorefakes"
	"github.com/cloudfoundry/bosh-agent/v2/mbus"
	"github.com/cloudfoundry/bosh-agent/v2/mbus/mbusfakes"
	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
	"github.com/cloudfoundry/bosh-agent/v2/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/v2/settings/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("HandlerProvider", func() {
	var (
		settingsService *fakesettings.FakeSettingsService
		platform        *platformfakes.FakePlatform
		auditLogger     *platformfakes.FakeAuditLogger
		logger          boshlog.Logger
		provider        mbus.HandlerProvider
		blobManager     *blobstorefakes.FakeBlobManagerInterface
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		logger = boshlog.NewLogger(boshlog.LevelNone)
		platform = &platformfakes.FakePlatform{}
		auditLogger = &platformfakes.FakeAuditLogger{}
		provider = mbus.NewHandlerProvider(settingsService, logger, auditLogger)
		blobManager = &blobstorefakes.FakeBlobManagerInterface{}
	})

	Describe("Get", func() {
		It("returns nats handler", func() {
			settingsService.Settings.Mbus = "nats://lol"
			handler, err := provider.Get(platform, blobManager)
			Expect(err).ToNot(HaveOccurred())

			connector := func(url string, options ...nats.Option) (mbus.NatsConnection, error) {
				return &mbusfakes.FakeNatsConnection{}, nil
			}
			expectedHandler := mbus.NewNatsHandler(settingsService, connector, logger, platform)
			Expect(reflect.TypeOf(handler)).To(Equal(reflect.TypeOf(expectedHandler)))
		})

		It("returns https handler when MBUS URL only specified", func() {
			mbusURL, err := url.Parse("https://foo:bar@lol")
			Expect(err).ToNot(HaveOccurred())

			settingsService.Settings.Mbus = "https://foo:bar@lol"
			handler, err := provider.Get(platform, blobManager)
			Expect(err).ToNot(HaveOccurred())
			expectedHandler := mbus.NewHTTPSHandler(mbusURL, settings.CertKeyPair{}, blobManager, logger, auditLogger)
			httpsHandler, ok := handler.(mbus.HTTPSHandler)
			Expect(ok).To(BeTrue())
			Expect(httpsHandler).To(Equal(expectedHandler))
		})

		It("returns https handler when MbusEnv are specified", func() {
			mbusURL, err := url.Parse("https://foo:bar@lol")
			Expect(err).ToNot(HaveOccurred())

			settingsService.Settings.Mbus = "https://foo:bar@lol"
			settingsService.Settings.Env.Bosh.Mbus.Cert.Certificate = "certificate-pem-block"
			settingsService.Settings.Env.Bosh.Mbus.Cert.PrivateKey = "private-key-pem-block"

			handler, err := provider.Get(platform, blobManager)
			expectedHandler := mbus.NewHTTPSHandler(
				mbusURL,
				settingsService.Settings.Env.Bosh.Mbus.Cert,
				blobManager,
				logger,
				auditLogger,
			)
			Expect(err).NotTo(HaveOccurred())
			httpsHandler, ok := handler.(mbus.HTTPSHandler)
			Expect(ok).To(BeTrue())
			Expect(reflect.DeepEqual(httpsHandler, expectedHandler)).To(BeTrue())
		})

		It("returns an error if not supported", func() {
			settingsService.Settings.Mbus = "unknown-scheme://lol"
			_, err := provider.Get(platform, blobManager)
			Expect(err).To(HaveOccurred())
		})
	})
})
