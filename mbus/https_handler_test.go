package mbus_test

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	. "github.com/cloudfoundry/bosh-agent/mbus"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshagentblobstore "github.com/cloudfoundry/bosh-agent/agent/blobstore"
	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	"github.com/cloudfoundry/bosh-agent/platform/fakes"
	"github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("HTTPSHandler", func() {
	var (
		serverURL       string
		tmpdir          string
		handler         HTTPSHandler
		receivedRequest boshhandler.Request
		httpClient      http.Client
		blobManager     boshagentblobstore.BlobManagerInterface
	)

	BeforeEach(func() {
		var err error
		tmpdir, err = ioutil.TempDir("", "mbus-http-handler-test")
		Expect(err).NotTo(HaveOccurred())

		serverURL = "https://user:pass@localhost:6900"
		blobManager, err = boshagentblobstore.NewBlobManager(tmpdir)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		handler.Stop()

		err := os.Chmod(tmpdir, 0700)
		Expect(err).NotTo(HaveOccurred())

		err = os.RemoveAll(tmpdir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when the agent creates the handler with custom cert", func() {
		BeforeEach(func() {
			configCert, err := ioutil.ReadFile("test_assets/custom_cert.pem")
			Expect(err).NotTo(HaveOccurred())
			configPrivateKey, err := ioutil.ReadFile("test_assets/custom_key.pem")
			Expect(err).NotTo(HaveOccurred())

			mbusKeyPair := settings.CertKeyPair{
				Certificate: string(configCert),
				PrivateKey:  string(configPrivateKey),
			}

			mbusURL, _ := url.Parse(serverURL)
			logger := boshlog.NewWriterLogger(boshlog.LevelDebug, GinkgoWriter)
			handler = NewHTTPSHandler(mbusURL, mbusKeyPair, blobManager, logger, fakes.NewFakeAuditLogger())

			go handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
				receivedRequest = req
				return boshhandler.NewValueResponse("expected value")
			})

			caCert, err := ioutil.ReadFile("test_assets/custom_ca.pem")
			Expect(err).NotTo(HaveOccurred())
			authority := x509.NewCertPool()
			Expect(authority.AppendCertsFromPEM(caCert)).To(BeTrue())

			httpTransport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: authority}}
			httpClient = http.Client{Timeout: 5 * time.Second, Transport: httpTransport}

			waitForServerToStart(serverURL, httpClient)
		})

		It("handshakes for a ping using that custom cert", func() {
			postBody := `{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`
			postPayload := strings.NewReader(postBody)

			httpResponse, err := httpClient.Post(serverURL+"/agent", "application/json", postPayload)
			for err != nil {
				httpResponse, err = httpClient.Post(serverURL+"/agent", "application/json", postPayload)
			}

			defer httpResponse.Body.Close()

			Expect(receivedRequest.ReplyTo).To(Equal("reply to me!"))
			Expect(receivedRequest.Method).To(Equal("ping"))
			Expect(receivedRequest.GetPayload()).To(Equal([]byte(postBody)))

			httpBody, readErr := ioutil.ReadAll(httpResponse.Body)
			Expect(readErr).ToNot(HaveOccurred())
			Expect(httpBody).To(Equal([]byte(`{"value":"expected value"}`)))
		})

		Describe("POST /agent", func() {
			It("receives request and responds", func() {
				postBody := `{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`
				postPayload := strings.NewReader(postBody)

				httpResponse, err := httpClient.Post(serverURL+"/agent", "application/json", postPayload)
				for err != nil {
					httpResponse, err = httpClient.Post(serverURL+"/agent", "application/json", postPayload)
				}

				defer httpResponse.Body.Close()

				Expect(receivedRequest.ReplyTo).To(Equal("reply to me!"))
				Expect(receivedRequest.Method).To(Equal("ping"))
				Expect(receivedRequest.GetPayload()).To(Equal([]byte(postBody)))

				httpBody, readErr := ioutil.ReadAll(httpResponse.Body)
				Expect(readErr).ToNot(HaveOccurred())
				Expect(httpBody).To(Equal([]byte(`{"value":"expected value"}`)))
			})

			Context("when incorrect http method is used", func() {
				It("returns a 404", func() {
					httpResponse, err := httpClient.Get(serverURL + "/agent")
					Expect(err).ToNot(HaveOccurred())
					Expect(httpResponse.StatusCode).To(Equal(404))
				})
			})
		})

		Describe("blob access", func() {
			Describe("GET /blobs", func() {
				It("returns data from file system", func() {
					err := blobManager.Write("123-456-789", strings.NewReader("Some data"))
					Expect(err).NotTo(HaveOccurred())

					httpResponse, err := httpClient.Get(serverURL + "/blobs/123-456-789")
					Expect(err).ToNot(HaveOccurred())
					defer httpResponse.Body.Close()

					httpBody, readErr := ioutil.ReadAll(httpResponse.Body)
					Expect(readErr).ToNot(HaveOccurred())
					Expect(httpResponse.StatusCode).To(Equal(200))
					Expect(httpBody).To(Equal([]byte("Some data")))
				})

				Context("when incorrect http method is used", func() {
					It("returns a 404", func() {
						postBody := `{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`
						postPayload := strings.NewReader(postBody)

						httpResponse, err := httpClient.Post(serverURL+"/blobs/123", "application/json", postPayload)
						Expect(err).ToNot(HaveOccurred())

						defer httpResponse.Body.Close()

						Expect(httpResponse.StatusCode).To(Equal(404))
					})
				})

				Context("when file does not exist", func() {
					It("returns a 404", func() {
						httpResponse, err := httpClient.Get(serverURL + "/blobs/a-file-that-does-not-exist")
						Expect(err).ToNot(HaveOccurred())

						defer httpResponse.Body.Close()
						Expect(httpResponse.StatusCode).To(Equal(404))
					})
				})
			})

			Describe("PUT /blobs", func() {
				It("updates the blob on the file system", func() {
					err := blobManager.Write("123-456-789", strings.NewReader("Some data"))
					Expect(err).NotTo(HaveOccurred())

					putBody := `Updated data`
					putPayload := strings.NewReader(putBody)

					request, err := http.NewRequest("PUT", serverURL+"/blobs/a5/123-456-789", putPayload)
					Expect(err).ToNot(HaveOccurred())

					httpResponse, err := httpClient.Do(request)
					Expect(err).ToNot(HaveOccurred())

					defer httpResponse.Body.Close()
					Expect(httpResponse.StatusCode).To(Equal(201))

					file, _, err := blobManager.Fetch("123-456-789")
					Expect(err).NotTo(HaveOccurred())
					defer file.Close()

					contents, err := ioutil.ReadAll(file)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(contents)).To(Equal("Updated data"))
				})

				Context("when an incorrect username and password is provided", func() {
					It("returns a 401", func() {
						err := blobManager.Write("123-456-789", strings.NewReader("Some data"))
						Expect(err).NotTo(HaveOccurred())

						putBody := `Updated data`
						putPayload := strings.NewReader(putBody)

						httpRequest, err := http.NewRequest("PUT", strings.Replace(serverURL, "pass", "wrong", -1)+"/blobs/a5/123-456-789", putPayload)
						Expect(err).NotTo(HaveOccurred())
						httpResponse, err := httpClient.Do(httpRequest)
						Expect(err).ToNot(HaveOccurred())

						defer httpResponse.Body.Close()

						Expect(httpResponse.StatusCode).To(Equal(401))
						Expect(httpResponse.Header.Get("WWW-Authenticate")).To(Equal(`Basic realm=""`))
					})
				})
			})
		})

		Describe("routing and auth", func() {
			Context("when an incorrect uri is specificed", func() {
				It("returns a 404", func() {
					postBody := `{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`
					postPayload := strings.NewReader(postBody)
					httpResponse, err := httpClient.Post(serverURL+"/bad_url", "application/json", postPayload)
					Expect(err).ToNot(HaveOccurred())

					defer httpResponse.Body.Close()

					Expect(httpResponse.StatusCode).To(Equal(404))
				})
			})

			Context("when an incorrect username/password was provided", func() {
				It("returns a 401", func() {
					postBody := `{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`
					postPayload := strings.NewReader(postBody)

					httpResponse, err := httpClient.Post(strings.Replace(serverURL, "pass", "wrong", -1)+"/agent", "application/json", postPayload)
					Expect(err).ToNot(HaveOccurred())

					defer httpResponse.Body.Close()

					Expect(httpResponse.StatusCode).To(Equal(401))
					Expect(httpResponse.Header.Get("WWW-Authenticate")).To(Equal(`Basic realm=""`))
				})
			})
		})
	})
})

func waitForServerToStart(serverURL string, httpClient http.Client) {
	Eventually(func() error {
		httpResponse, err := httpClient.Get(serverURL + "/healthz")
		if err == nil {
			httpResponse.Body.Close()
		}
		return err
	}, time.Second*5).Should(Succeed())
}
