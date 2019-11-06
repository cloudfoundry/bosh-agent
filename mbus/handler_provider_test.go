package mbus_test

import (
	gourl "net/url"
	"reflect"
	"time"

	. "github.com/cloudfoundry/bosh-agent/mbus"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	"github.com/cloudfoundry/bosh-agent/settings"
	"github.com/cloudfoundry/yagnats"

	fakeblobstore "github.com/cloudfoundry/bosh-agent/agent/blobstore/blobstorefakes"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("HandlerProvider", func() {
	var (
		settingsService *fakesettings.FakeSettingsService
		platform        *platformfakes.FakePlatform
		auditLogger     *platformfakes.FakeAuditLogger
		logger          boshlog.Logger
		provider        HandlerProvider
		blobManager     *fakeblobstore.FakeBlobManagerInterface
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		logger = boshlog.NewLogger(boshlog.LevelNone)
		platform = &platformfakes.FakePlatform{}
		auditLogger = &platformfakes.FakeAuditLogger{}
		provider = NewHandlerProvider(settingsService, logger, auditLogger)
		blobManager = &fakeblobstore.FakeBlobManagerInterface{}
	})

	Describe("Get", func() {
		It("returns nats handler", func() {
			settingsService.Settings.Mbus = "nats://lol"
			handler, err := provider.Get(platform, blobManager)
			Expect(err).ToNot(HaveOccurred())

			// yagnats.NewClient returns new object every time
			expectedHandler := NewNatsHandler(settingsService, yagnats.NewClient(), logger, platform, time.Second, 1*time.Minute)
			Expect(reflect.TypeOf(handler)).To(Equal(reflect.TypeOf(expectedHandler)))
		})

		It("returns https handler when MBUS URL only specified", func() {
			mbusURL, err := gourl.Parse("https://foo:bar@lol")
			Expect(err).ToNot(HaveOccurred())

			settingsService.Settings.Mbus = "https://foo:bar@lol"
			handler, err := provider.Get(platform, blobManager)
			Expect(err).ToNot(HaveOccurred())
			expectedHandler := NewHTTPSHandler(mbusURL, settings.CertKeyPair{}, blobManager, logger, auditLogger)
			httpsHandler, ok := handler.(HTTPSHandler)
			Expect(ok).To(BeTrue())
			Expect(httpsHandler).To(Equal(expectedHandler))
		})

		It("returns https handler when MbusEnv are specified", func() {
			mbusURL, err := gourl.Parse("https://foo:bar@lol")
			Expect(err).ToNot(HaveOccurred())

			settingsService.Settings.Mbus = "https://foo:bar@lol"
			settingsService.Settings.Env.Bosh.Mbus.Cert.Certificate = "certificate-pem-block"
			settingsService.Settings.Env.Bosh.Mbus.Cert.PrivateKey = "private-key-pem-block"

			handler, err := provider.Get(platform, blobManager)
			expectedHandler := NewHTTPSHandler(
				mbusURL,
				settingsService.Settings.Env.Bosh.Mbus.Cert,
				blobManager,
				logger,
				auditLogger,
			)
			httpsHandler, ok := handler.(HTTPSHandler)
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
