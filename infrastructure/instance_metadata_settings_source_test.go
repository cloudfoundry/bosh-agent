package infrastructure_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/infrastructure"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("InstanceMetadataSettingsSource", describeInstanceMetadataSettingsSource)

func describeInstanceMetadataSettingsSource() {
	var (
		metadataHeaders map[string]string
		settingsPath    string
		platform        *platformfakes.FakePlatform
		logger          boshlog.Logger
		metadataSource  *infrastructure.InstanceMetadataSettingsSource
	)

	BeforeEach(func() {
		metadataHeaders = make(map[string]string)
		metadataHeaders["key"] = "value"
		settingsPath = "/computeMetadata/v1/instance/attributes/bosh_settings"
		platform = &platformfakes.FakePlatform{}
		logger = boshlog.NewLogger(boshlog.LevelNone)
		metadataSource = infrastructure.NewInstanceMetadataSettingsSource("http://fake-metadata-host", metadataHeaders, settingsPath, platform, logger)
	})

	Describe("PublicSSHKeyForUsername", func() {
		It("returns an empty string", func() {
			publicKey, err := metadataSource.PublicSSHKeyForUsername("fake-username")
			Expect(err).ToNot(HaveOccurred())
			Expect(publicKey).To(Equal(""))
		})
	})

	Describe("Settings", func() {
		var (
			ts *httptest.Server
		)

		handlerFunc := func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()

			Expect(r.Method).To(Equal("GET"))
			Expect(r.URL.Path).To(Equal(settingsPath))
			Expect(r.Header.Get("key")).To(Equal("value"))

			_, err := w.Write([]byte(`{"agent_id": "123"}`))
			Expect(err).NotTo(HaveOccurred())
		}

		BeforeEach(func() {
			handler := http.HandlerFunc(handlerFunc)
			ts = httptest.NewServer(handler)
			metadataSource = infrastructure.NewInstanceMetadataSettingsSource(ts.URL, metadataHeaders, settingsPath, platform, logger)
		})

		AfterEach(func() {
			ts.Close()
		})

		It("returns settings read from the instance metadata endpoint", func() {
			settings, err := metadataSource.Settings()
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.AgentID).To(Equal("123"))
		})

		It("returns an error if reading from the instance metadata endpoint fails", func() {
			metadataSource = infrastructure.NewInstanceMetadataSettingsSourceWithoutRetryDelay("bad-registry-endpoint", metadataHeaders, settingsPath, platform, logger)
			_, err := metadataSource.Settings()
			Expect(err).To(HaveOccurred())
		})
	})
}
