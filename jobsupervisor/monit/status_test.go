package monit_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
)

var _ = Describe("status", func() {
	Describe("ServicesInGroup", func() {
		It("returns list of service", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := io.Copy(w, bytes.NewReader(readFixture(statusWithMultipleServiceFixturePath)))
				Expect(err).ToNot(HaveOccurred())
				Expect(r.Method).To(Equal("GET"))
				Expect(r.URL.Path).To(Equal("/_status2"))
				Expect(r.URL.Query().Get("format")).To(Equal("xml"))
			})

			ts := httptest.NewServer(handler)
			defer ts.Close()

			logger := boshlog.NewLogger(boshlog.LevelNone)

			httpClient := http.DefaultClient

			client := NewHTTPClient(
				ts.Listener.Addr().String(),
				"fake-user",
				"fake-pass",
				httpClient,
				httpClient,
				logger,
			)

			status, err := client.Status()
			Expect(err).ToNot(HaveOccurred())

			expectedServices := []Service{
				Service{Name: "running-service", Monitored: true, Status: "running"},
				Service{Name: "unmonitored-service", Monitored: false, Status: "unknown"},
				Service{Name: "starting-service", Monitored: true, Status: "starting"},
				Service{Name: "failing-service", Monitored: true, Status: "failing"},
			}

			services := status.ServicesInGroup("vcap")
			Expect(len(services)).To(Equal(len(expectedServices)))

			for i, expectedService := range expectedServices {
				Expect(expectedService).To(Equal(services[i]))
			}
		})
	})
})
