package monit_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	. "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/monit"
)

var _ = Describe("status", func() {
	Describe("ServicesInGroup", func() {
		It("returns a list of services", func() {
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
				Service{Monitored: false, Name: "unmonitored-start-pending", Status: "unknown", Pending: true},
				Service{Monitored: true, Name: "initializing", Status: "starting", Pending: false},
				Service{Monitored: true, Name: "running", Status: "running", Pending: false},
				Service{Monitored: true, Name: "running-stop-pending", Status: "running", Pending: true},
				Service{Monitored: false, Name: "unmonitored-stop-pending", Status: "unknown", Pending: true},
				Service{Monitored: false, Name: "unmonitored", Status: "unknown", Pending: false},
				Service{Monitored: false, Name: "stopped", Status: "unknown", Pending: false},
				Service{Monitored: true, Name: "failing", Status: "failing", Pending: false},
			}

			services := status.ServicesInGroup("vcap")
			Expect(len(services)).To(Equal(len(expectedServices)))

			for i, expectedService := range expectedServices {
				Expect(expectedService).To(Equal(services[i]))
			}
		})

		It("returns list of detailed service", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := io.Copy(w, bytes.NewReader(readFixture(statusFixturePath)))
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
				Service{
					Name:                 "dummy",
					Monitored:            true,
					Errored:              false,
					Pending:              false,
					Status:               "running",
					StatusMessage:        "",
					Uptime:               880183,
					MemoryPercentTotal:   0,
					MemoryKilobytesTotal: 4004,
					CPUPercentTotal:      0,
				},
			}

			services := status.ServicesInGroup("vcap")
			Expect(len(services)).To(Equal(len(expectedServices)))

			for i, expectedService := range expectedServices {
				Expect(expectedService).To(Equal(services[i]))
			}
		})

	})
})
