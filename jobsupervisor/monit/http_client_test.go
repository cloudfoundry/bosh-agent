package monit_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	fakehttp "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/http/fakes"
	boshlog "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
)

var _ = Describe("httpClient", func() {
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

			client := newRealClient(ts.Listener.Addr().String())

			err := client.StartService("test-service")
			Expect(err).ToNot(HaveOccurred())
			Expect(calledMonit).To(BeTrue())
		})

		It("uses the shortClient to send a start request", func() {
			shortClient := fakehttp.NewFakeClient()
			longClient := fakehttp.NewFakeClient()
			client := newFakeClient(shortClient, longClient)

			shortClient.StatusCode = 200

			err := client.StartService("test-service")
			Expect(err).ToNot(HaveOccurred())

			Expect(shortClient.CallCount).To(Equal(1))
			Expect(longClient.CallCount).To(Equal(0))

			req := shortClient.Requests[0]
			Expect(req.URL.Host).To(Equal("agent.example.com"))
			Expect(req.URL.Path).To(Equal("/test-service"))
			Expect(req.Method).To(Equal("POST"))

			content := shortClient.RequestBodies[0]
			Expect(content).To(Equal("action=start"))
		})
	})

	Describe("StopService", func() {
		It("uses the longClient to send the unmonitor/stop requests, shortClient to send a status request", func() {
			shortClient := fakehttp.NewFakeClient()
			longClient := fakehttp.NewFakeClient()
			client := newFakeClient(shortClient, longClient)

			longClient.StatusCode = 200

			client.StopService("test-service")

			Expect(longClient.CallCount).To(Equal(2))
			Expect(longClient.RequestBodies[0]).To(Equal("action=unmonitor"))
			Expect(longClient.RequestBodies[1]).To(Equal("action=stop"))

			Expect(shortClient.CallCount).To(Equal(1))
			Expect(shortClient.Requests[0].URL.Path).To(Equal("/_status2"))
		})

		It("waits for the monit status to indicate the service has stopped", func() {
			tries := 0
			httpCalls := []map[string]string{}

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var resBody []byte

				requestData := make(map[string]string)

				if r.URL.Path == "/_status2" {
					Expect(r.Method).To(Equal("GET"))

					tries++
					requestData["action"] = "status"

					switch {
					case tries == 1:
						resBody = readFixture("test_assets/monit_status_running.xml")
					case tries == 2:
						resBody = readFixture("test_assets/monit_status_stopping.xml")
					case tries == 3:
						resBody = readFixture("test_assets/monit_status_stopped.xml")
					}
					w.Write(resBody)
				} else {
					Expect(r.Method).To(Equal("POST"))
					requestData["action"] = r.PostFormValue("action")
				}

				requestData["url"] = r.URL.Path

				expectedAuthEncoded := base64.URLEncoding.EncodeToString([]byte("fake-user:fake-pass"))
				Expect(r.Header.Get("Authorization")).To(Equal(fmt.Sprintf("Basic %s", expectedAuthEncoded)))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))

				httpCalls = append(httpCalls, requestData)
			})

			ts := httptest.NewServer(handler)
			defer ts.Close()

			client := newRealClient(ts.Listener.Addr().String())

			err := client.StopService("test-service")
			Expect(err).ToNot(HaveOccurred())

			Expect(httpCalls[0]["url"]).To(Equal("/test-service"))
			Expect(httpCalls[0]["action"]).To(Equal("unmonitor"))

			Expect(httpCalls[1]["url"]).To(Equal("/test-service"))
			Expect(httpCalls[1]["action"]).To(Equal("stop"))

			Expect(httpCalls[2]["url"]).To(Equal("/_status2"))
			Expect(httpCalls[2]["action"]).To(Equal("status"))

			Expect(httpCalls[3]["url"]).To(Equal("/_status2"))
			Expect(httpCalls[3]["action"]).To(Equal("status"))

			Expect(httpCalls[4]["url"]).To(Equal("/_status2"))
			Expect(httpCalls[4]["action"]).To(Equal("status"))

			Expect(len(httpCalls)).To(Equal(5))
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

			client := newRealClient(ts.Listener.Addr().String())

			err := client.UnmonitorService("test-service")
			Expect(err).ToNot(HaveOccurred())
			Expect(calledMonit).To(BeTrue())
		})

		It("uses the longClient to send an unmonitor request", func() {
			shortClient := fakehttp.NewFakeClient()
			longClient := fakehttp.NewFakeClient()
			client := newFakeClient(shortClient, longClient)

			longClient.StatusCode = 200

			err := client.UnmonitorService("test-service")
			Expect(err).ToNot(HaveOccurred())

			Expect(shortClient.CallCount).To(Equal(0))
			Expect(longClient.CallCount).To(Equal(1))

			req := longClient.Requests[0]
			Expect(req.URL.Host).To(Equal("agent.example.com"))
			Expect(req.URL.Path).To(Equal("/test-service"))
			Expect(req.Method).To(Equal("POST"))

			content := longClient.RequestBodies[0]
			Expect(content).To(Equal("action=unmonitor"))
		})
	})

	Describe("ServicesInGroup", func() {
		It("services in group", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := io.Copy(w, bytes.NewReader(readFixture(statusFixturePath)))
				Expect(err).ToNot(HaveOccurred())
				Expect(r.Method).To(Equal("GET"))
				Expect(r.URL.Path).To(Equal("/_status2"))
				Expect(r.URL.Query().Get("format")).To(Equal("xml"))
			})
			ts := httptest.NewServer(handler)
			defer ts.Close()

			client := newRealClient(ts.Listener.Addr().String())

			services, err := client.ServicesInGroup("vcap")
			Expect(err).ToNot(HaveOccurred())
			Expect(services).To(Equal([]string{"dummy"}))
		})
	})

	Describe("Status", func() {
		It("decode status", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := io.Copy(w, bytes.NewReader(readFixture(statusFixturePath)))
				Expect(err).ToNot(HaveOccurred())
				Expect(r.Method).To(Equal("GET"))
				Expect(r.URL.Path).To(Equal("/_status2"))
				Expect(r.URL.Query().Get("format")).To(Equal("xml"))
			})
			ts := httptest.NewServer(handler)
			defer ts.Close()

			client := newRealClient(ts.Listener.Addr().String())

			status, err := client.Status()
			Expect(err).ToNot(HaveOccurred())

			dummyServices := status.ServicesInGroup("vcap")
			Expect(len(dummyServices)).To(Equal(1))
		})

		It("uses the shortClient to send a status request and parses the response xml", func() {
			shortClient := fakehttp.NewFakeClient()
			longClient := fakehttp.NewFakeClient()
			client := newFakeClient(shortClient, longClient)

			shortClient.StatusCode = 200
			shortClient.SetMessage(string(readFixture(statusWithMultipleServiceFixturePath)))

			status, err := client.Status()
			Expect(err).ToNot(HaveOccurred())

			expectedServices := []Service{
				Service{Monitored: true, Status: "running"},
				Service{Monitored: false, Status: "unknown"},
				Service{Monitored: true, Status: "starting"},
				Service{Monitored: true, Status: "failing"},
			}

			services := status.ServicesInGroup("vcap")
			Expect(len(services)).To(Equal(len(expectedServices)))

			Expect(shortClient.CallCount).To(Equal(1))
			Expect(longClient.CallCount).To(Equal(0))

			req := shortClient.Requests[0]
			Expect(req.URL.Host).To(Equal("agent.example.com"))
			Expect(req.URL.Path).To(Equal("/_status2"))
			Expect(req.Method).To(Equal("GET"))
		})
	})
})

func newRealClient(url string) Client {
	logger := boshlog.NewLogger(boshlog.LevelNone)

	return NewHTTPClient(
		url,
		"fake-user",
		"fake-pass",
		http.DefaultClient,
		http.DefaultClient,
		logger,
	)
}

func newFakeClient(shortClient, longClient *fakehttp.FakeClient) Client {
	logger := boshlog.NewLogger(boshlog.LevelNone)

	return NewHTTPClient(
		"agent.example.com",
		"fake-user",
		"fake-pass",
		shortClient,
		longClient,
		logger,
	)
}
