package monit_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakehttp "github.com/cloudfoundry/bosh-agent/http/fakes"
	. "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"

	faketime "github.com/cloudfoundry/bosh-agent/time/fakes"
)

func init() {
	Describe("httpClient", func() {
		var (
			logger      = boshlog.NewLogger(boshlog.LevelNone)
			timeService = &faketime.FakeService{}
		)

		It("services in group returns services when found", func() {})

		It("services in group errors when not found", func() {})

		Describe("StartService", func() {
			It("start service", func() {
				var calledMonit bool

				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calledMonit = true
					Expect(r.Method).To(Equal("POST"))
					Expect(r.URL.Path).To(Equal("/test-service"))
					Expect(r.PostFormValue("action")).To(Equal("start"))
					Expect(r.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))

					expectedAuthEncoded := base64.URLEncoding.EncodeToString([]byte("fake-user:fake-pass"))
					Expect(r.Header.Get("Authorization")).To(Equal(fmt.Sprintf("Basic %s", expectedAuthEncoded)))
				})
				ts := httptest.NewServer(handler)
				defer ts.Close()

				client := NewHTTPClient(ts.Listener.Addr().String(), "fake-user", "fake-pass", http.DefaultClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.StartService("test-service")
				Expect(err).ToNot(HaveOccurred())
				Expect(calledMonit).To(BeTrue())
			})

			It("start service retries when non200 response", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.StatusCode = 500
				fakeHTTPClient.SetMessage("fake error message")

				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.StartService("test-service")
				Expect(fakeHTTPClient.CallCount).To(Equal(10))
				Expect(err).To(HaveOccurred())
			})

			It("retries using the default interval", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.StatusCode = 500
				fakeHTTPClient.SetMessage("fake error message")

				defaultDelay := 1 * time.Millisecond
				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, defaultDelay, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.StartService("test-service")
				Expect(timeService.SleepDuration).To(Equal(defaultDelay))
				Expect(err).To(HaveOccurred())
			})

			It("start service retries when connection refused", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.SetNilResponse()
				fakeHTTPClient.Error = errors.New("some error")

				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.StartService("test-service")
				Expect(fakeHTTPClient.CallCount).To(Equal(10))
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("StopService", func() {
			It("stop service", func() {
				var calledMonit bool

				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calledMonit = true
					Expect(r.Method).To(Equal("POST"))
					Expect(r.URL.Path).To(Equal("/test-service"))
					Expect(r.PostFormValue("action")).To(Equal("stop"))
					Expect(r.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))

					expectedAuthEncoded := base64.URLEncoding.EncodeToString([]byte("fake-user:fake-pass"))
					Expect(r.Header.Get("Authorization")).To(Equal(fmt.Sprintf("Basic %s", expectedAuthEncoded)))
				})
				ts := httptest.NewServer(handler)
				defer ts.Close()

				client := NewHTTPClient(ts.Listener.Addr().String(), "fake-user", "fake-pass", http.DefaultClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 20, 300, 300, logger, timeService)

				err := client.StopService("test-service")
				Expect(err).ToNot(HaveOccurred())
				Expect(calledMonit).To(BeTrue())
			})

			It("stop service retries when non200 response the specified number of times", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.StatusCode = 500
				fakeHTTPClient.SetMessage("fake error message")

				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.StopService("test-service")
				Expect(fakeHTTPClient.CallCount).To(Equal(20))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake error message"))
			})

			It("stop service retries when connection refused", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.SetNilResponse()
				fakeHTTPClient.Error = errors.New("some error")

				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.StopService("test-service")
				Expect(fakeHTTPClient.CallCount).To(Equal(20))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("some error"))
			})

			It("stop service retries with the specified stop delay interval for 503 response", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.StatusCode = 503
				fakeHTTPClient.SetMessage("Service Unavailable")

				stopDelay := 2 * time.Millisecond
				defaultDelay := 1 * time.Millisecond
				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, defaultDelay, stopDelay, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.StopService("test-service")
				Expect(timeService.SleepDuration).To(Equal(stopDelay))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Service Unavailable"))
			})

			It("stop service switches to default delay if error changes from 503", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.StatusCode = 503
				fakeHTTPClient.NewStatusCode = 500
				fakeHTTPClient.RetriesBeforeChange = 5
				fakeHTTPClient.SetMessage("Service Unavailable")

				stopDelay := 2 * time.Millisecond
				defaultDelay := 1 * time.Millisecond
				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, defaultDelay, stopDelay, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.StopService("test-service")
				Expect(timeService.SleepDuration).To(Equal(defaultDelay))
				Expect(fakeHTTPClient.CallCount).To(Equal(24))
				Expect(err).To(HaveOccurred())
			})

			It("does not reset more than once when receiving subsequent 503 errors", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.KeepFlippingStatusCode(5, 503, 500)
				fakeHTTPClient.SetMessage("Service Unavailable")

				stopDelay := 2 * time.Millisecond
				defaultDelay := 1 * time.Millisecond
				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, defaultDelay, stopDelay, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.StopService("test-service")
				Expect(timeService.SleepDuration).To(Equal(defaultDelay))
				Expect(fakeHTTPClient.CallCount).To(Equal(24))
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("UnmonitorService", func() {
			It("issues a call to unmonitor service by name", func() {
				var calledMonit bool

				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calledMonit = true
					Expect(r.Method).To(Equal("POST"))
					Expect(r.URL.Path).To(Equal("/test-service"))
					Expect(r.PostFormValue("action")).To(Equal("unmonitor"))
					Expect(r.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))

					expectedAuthEncoded := base64.URLEncoding.EncodeToString([]byte("fake-user:fake-pass"))
					Expect(r.Header.Get("Authorization")).To(Equal(fmt.Sprintf("Basic %s", expectedAuthEncoded)))
				})

				ts := httptest.NewServer(handler)
				defer ts.Close()

				client := NewHTTPClient(ts.Listener.Addr().String(), "fake-user", "fake-pass", http.DefaultClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 20, 300, 300, logger, timeService)

				err := client.UnmonitorService("test-service")
				Expect(err).ToNot(HaveOccurred())
				Expect(calledMonit).To(BeTrue())
			})

			It("retries when non200 response the specified number of times", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.StatusCode = 500
				fakeHTTPClient.SetMessage("fake-http-response-message")

				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.UnmonitorService("test-service")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-http-response-message"))

				Expect(fakeHTTPClient.CallCount).To(Equal(30))
			})

			It("retries when connection refused", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.SetNilResponse()
				fakeHTTPClient.Error = errors.New("fake-http-error")

				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.UnmonitorService("test-service")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-http-error"))

				Expect(fakeHTTPClient.CallCount).To(Equal(30))
			})

			It("unmonitor service retries with the specified unmonitor delay interval for 503 response", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.StatusCode = 503
				fakeHTTPClient.SetMessage("Service unavailable")

				unmonitorDelay := 3 * time.Millisecond
				defaultDelay := 1 * time.Millisecond
				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, defaultDelay, 2*time.Millisecond, unmonitorDelay, 20, 300, 300, logger, timeService)

				err := client.UnmonitorService("test-service")
				Expect(timeService.SleepDuration).To(Equal(unmonitorDelay))
				Expect(err).To(HaveOccurred())
			})

			It("unmonitor service switches to default delay if error changes from 503", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.StatusCode = 503
				fakeHTTPClient.NewStatusCode = 500
				fakeHTTPClient.RetriesBeforeChange = 5
				fakeHTTPClient.SetMessage("Service Unavailable")

				unmonitorDelay := 3 * time.Millisecond
				defaultDelay := 1 * time.Millisecond
				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, defaultDelay, 2*time.Millisecond, unmonitorDelay, 10, 20, 30, logger, timeService)

				err := client.UnmonitorService("test-service")
				Expect(timeService.SleepDuration).To(Equal(defaultDelay))
				Expect(fakeHTTPClient.CallCount).To(Equal(34))
				Expect(err).To(HaveOccurred())
			})

			It("does not reset more than once when receiving subsequent 503 errors", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.KeepFlippingStatusCode(5, 503, 500)
				fakeHTTPClient.SetMessage("Service Unavailable")

				stopDelay := 2 * time.Millisecond
				defaultDelay := 1 * time.Millisecond
				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, defaultDelay, stopDelay, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.UnmonitorService("test-service")
				Expect(timeService.SleepDuration).To(Equal(defaultDelay))
				Expect(fakeHTTPClient.CallCount).To(Equal(34))
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("ServicesInGroup", func() {
			It("services in group", func() {
				monitStatusFilePath, _ := filepath.Abs("../../Fixtures/monit_status.xml")
				Expect(monitStatusFilePath).ToNot(BeNil())

				file, err := os.Open(monitStatusFilePath)
				Expect(err).ToNot(HaveOccurred())
				defer file.Close()

				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					io.Copy(w, file)
					Expect(r.Method).To(Equal("GET"))
					Expect(r.URL.Path).To(Equal("/_status2"))
					Expect(r.URL.Query().Get("format")).To(Equal("xml"))
				})
				ts := httptest.NewServer(handler)
				defer ts.Close()

				client := NewHTTPClient(ts.Listener.Addr().String(), "fake-user", "fake-pass", http.DefaultClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 20, 300, 300, logger, timeService)

				services, err := client.ServicesInGroup("vcap")
				Expect(err).ToNot(HaveOccurred())
				Expect([]string{"dummy"}).To(Equal(services))
			})
		})

		Describe("Status", func() {
			It("decode status", func() {
				monitStatusFilePath, _ := filepath.Abs("../../Fixtures/monit_status.xml")
				Expect(monitStatusFilePath).ToNot(BeNil())

				file, err := os.Open(monitStatusFilePath)
				Expect(err).ToNot(HaveOccurred())
				defer file.Close()

				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					io.Copy(w, file)
					Expect(r.Method).To(Equal("GET"))
					Expect(r.URL.Path).To(Equal("/_status2"))
					Expect(r.URL.Query().Get("format")).To(Equal("xml"))
				})
				ts := httptest.NewServer(handler)
				defer ts.Close()

				client := NewHTTPClient(ts.Listener.Addr().String(), "fake-user", "fake-pass", http.DefaultClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				status, err := client.Status()
				Expect(err).ToNot(HaveOccurred())
				dummyServices := status.ServicesInGroup("vcap")
				Expect(1).To(Equal(len(dummyServices)))
			})

			It("status retries when non200 response", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.StatusCode = 500
				fakeHTTPClient.SetMessage("fake error message")

				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				_, err := client.Status()
				Expect(fakeHTTPClient.CallCount).To(Equal(10))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake error message"))
			})

			It("status retries when connection refused", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.SetNilResponse()
				fakeHTTPClient.Error = errors.New("some error")

				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, 1*time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				err := client.StartService("hello")
				Expect(fakeHTTPClient.CallCount).To(Equal(10))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("some error"))

				for _, req := range fakeHTTPClient.RequestBodies {
					Expect(req).To(Equal("action=start"))
				}
			})

			It("status retries with the default interval", func() {
				fakeHTTPClient := fakehttp.NewFakeClient()
				fakeHTTPClient.StatusCode = 500
				fakeHTTPClient.SetMessage("fake error message")

				defaultDelay := 1 * time.Millisecond
				client := NewHTTPClient("agent.example.com", "fake-user", "fake-pass", fakeHTTPClient, defaultDelay, 2*time.Millisecond, 3*time.Millisecond, 10, 20, 30, logger, timeService)

				_, err := client.Status()
				Expect(timeService.SleepDuration).To(Equal(defaultDelay))
				Expect(err).To(HaveOccurred())
			})
		})
	})
}
