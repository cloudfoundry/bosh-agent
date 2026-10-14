package mbus

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/agentpassword"
	"github.com/cloudfoundry/bosh-agent/v2/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("HTTPS dispatcher authentication", func() {
	const password = "correct-horse-battery-staple"

	Context("when configured with an agentpassword verifier URL", func() {
		var dispatcher *HTTPSDispatcher

		BeforeEach(func() {
			rawURL := "https://vcap:" + password + "@127.0.0.1:6868"
			hashedURL, err := agentpassword.HashURL(rawURL)
			Expect(err).NotTo(HaveOccurred())

			u, err := url.Parse(hashedURL)
			Expect(err).NotTo(HaveOccurred())

			dispatcher, err = NewHTTPSDispatcher(u, settings.CertKeyPair{}, boshlog.NewLogger(boshlog.LevelNone))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200/ok on /agent and /blobs/x with the correct password", func() {
			for _, path := range []string{"/agent", "/blobs/test-blob"} {
				dispatcher.AddRoute(path, func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				})

				req := httptest.NewRequest(http.MethodPost, "https://localhost"+path, nil)
				req.SetBasicAuth("vcap", password)
				rec := httptest.NewRecorder()
				dispatcher.mux.ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
			}
		})

		It("returns 401 for wrong password, empty password, wrong username, and password longer than 1024", func() {
			dispatcher.AddRoute("/agent", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			testCases := []struct {
				name     string
				username string
				password string
			}{
				{"wrong password", "vcap", "incorrect-password"},
				{"empty password", "vcap", ""},
				{"wrong username", "wrong-user", password},
				{"password longer than 1024", "vcap", strings.Repeat("x", 1025)},
			}

			for _, tc := range testCases {
				By(tc.name)
				req := httptest.NewRequest(http.MethodPost, "https://localhost/agent", nil)
				req.SetBasicAuth(tc.username, tc.password)
				rec := httptest.NewRecorder()
				dispatcher.mux.ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			}
		})

		It("returns 401 and WWW-Authenticate for requests with missing or non-Basic Authorization header", func() {
			dispatcher.AddRoute("/agent", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Request with no Authorization header
			reqNoAuth := httptest.NewRequest(http.MethodPost, "https://localhost/agent", nil)
			recNoAuth := httptest.NewRecorder()
			dispatcher.mux.ServeHTTP(recNoAuth, reqNoAuth)

			Expect(recNoAuth.Code).To(Equal(http.StatusUnauthorized))
			Expect(recNoAuth.Header().Get("WWW-Authenticate")).To(Equal(`Basic realm=""`))

			// Request with non-Basic Authorization header
			reqBearer := httptest.NewRequest(http.MethodPost, "https://localhost/agent", nil)
			reqBearer.Header.Set("Authorization", "Bearer x")
			recBearer := httptest.NewRecorder()
			dispatcher.mux.ServeHTTP(recBearer, reqBearer)

			Expect(recBearer.Code).To(Equal(http.StatusUnauthorized))
			Expect(recBearer.Header().Get("WWW-Authenticate")).To(Equal(`Basic realm=""`))
		})
	})

	Context("when configured with a legacy plaintext password", func() {
		It("still works and authenticates valid requests", func() {
			u, err := url.Parse("https://vcap:legacy-password@127.0.0.1:6868")
			Expect(err).NotTo(HaveOccurred())

			dispatcher, err := NewHTTPSDispatcher(u, settings.CertKeyPair{}, boshlog.NewLogger(boshlog.LevelNone))
			Expect(err).NotTo(HaveOccurred())

			dispatcher.AddRoute("/agent", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Valid credentials
			req := httptest.NewRequest(http.MethodPost, "https://localhost/agent", nil)
			req.SetBasicAuth("vcap", "legacy-password")
			rec := httptest.NewRecorder()
			dispatcher.mux.ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))

			// Invalid credentials
			reqBad := httptest.NewRequest(http.MethodPost, "https://localhost/agent", nil)
			reqBad.SetBasicAuth("vcap", "wrong-password")
			recBad := httptest.NewRecorder()
			dispatcher.mux.ServeHTTP(recBad, reqBad)
			Expect(recBad.Code).To(Equal(http.StatusUnauthorized))

			// Missing Authorization header
			reqNoAuth := httptest.NewRequest(http.MethodPost, "https://localhost/agent", nil)
			recNoAuth := httptest.NewRecorder()
			dispatcher.mux.ServeHTTP(recNoAuth, reqNoAuth)
			Expect(recNoAuth.Code).To(Equal(http.StatusUnauthorized))
		})
	})

	Context("when configured with a malformed verifier", func() {
		It("fails closed and returns an error from NewHTTPSDispatcher", func() {
			u, err := url.Parse("https://vcap:bosh-hmac-sha256$bad@127.0.0.1:6868")
			Expect(err).NotTo(HaveOccurred())

			dispatcher, err := NewHTTPSDispatcher(u, settings.CertKeyPair{}, boshlog.NewLogger(boshlog.LevelNone))
			Expect(err).To(HaveOccurred())
			Expect(dispatcher).To(BeNil())
		})
	})
})
