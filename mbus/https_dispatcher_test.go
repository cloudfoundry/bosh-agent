package mbus_test

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakelogger "github.com/cloudfoundry/bosh-agent/logger/fakes"
	"github.com/cloudfoundry/bosh-agent/mbus"
	"github.com/cloudfoundry/bosh-agent/settings"
)

const targetURL = "https://user:pass@127.0.0.1:7789"

const agentCert = `-----BEGIN CERTIFICATE-----
MIIC3zCCAcegAwIBAgIBADANBgkqhkiG9w0BAQUFADAzMQswCQYDVQQGEwJVUzEQ
MA4GA1UECgwHUGl2b3RhbDESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTEzMTIwMTIy
MTEzMloXDTE2MTIwMTIyMTEzMlowMzELMAkGA1UEBhMCVVMxEDAOBgNVBAoMB1Bp
dm90YWwxEjAQBgNVBAMMCWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEBBQADggEP
ADCCAQoCggEBALra3YIx0O6PLcQsHAXbFzJye1M/Am3gCMcX5BTui+I7vfyMXG4w
FyXHLf9lWwe23ypvP4c1+LNTq7UTTZuidrnadlADgdDned0C09bOKv5Nzk466XTR
fNyBEyxaZzHQLa9SmDLgq1GUun8oEsxZ+uXhMq6kikRu5LBCChtVCW6LzG/FE1qm
jSSH6iaOwk2yQxKVqUKNPfz1PqtRgaUBjVWrh2+Wf22KzOTORouBOrSfdxep1Cjz
lQWt2W5l05dvf2vZTlaqDCk8PBF36FWPlwmZxRsHGACVuckl3yJ69jIaa+i+mK3k
cfi05ZafWeFwm21ahqzwK/kGsK1ofPHKxE8CAwEAATANBgkqhkiG9w0BAQUFAAOC
AQEAD1VzwtWCx32pQi5l0oFfjWqktnqfhs/Rr0ZpwacBBXHBvKuz9ENXmblt5pZu
JP7je+uXQD+da/oVhl0US2L0upIdMmD1utVXYHfRji5r/tIPl2SEKTrFiNZR1Wp6
J0nE/BW7nm41dXRBIAZR71yproaQrt1tFDFZvdfhwHGLC51L6toOhk/7S604sxbk
qV0tzT+VaR4hh09FEt9xGmB/3yFh329Yib8ScT94nKzSzoNoDp4Ms/smFhF4lUio
7SD2+b2/nt8Mcz7q58nYvZteipRrmkOFszlNF5dU31FjvRLITn0bhiOOFRD3qAou
cSOr1qwsAKuu6MzYNh2ubsLvfg==
-----END CERTIFICATE-----`

const agentKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAutrdgjHQ7o8txCwcBdsXMnJ7Uz8CbeAIxxfkFO6L4ju9/Ixc
bjAXJcct/2VbB7bfKm8/hzX4s1OrtRNNm6J2udp2UAOB0Od53QLT1s4q/k3OTjrp
dNF83IETLFpnMdAtr1KYMuCrUZS6fygSzFn65eEyrqSKRG7ksEIKG1UJbovMb8UT
WqaNJIfqJo7CTbJDEpWpQo09/PU+q1GBpQGNVauHb5Z/bYrM5M5Gi4E6tJ93F6nU
KPOVBa3ZbmXTl29/a9lOVqoMKTw8EXfoVY+XCZnFGwcYAJW5ySXfInr2Mhpr6L6Y
reRx+LTllp9Z4XCbbVqGrPAr+QawrWh88crETwIDAQABAoIBAFh3DrB5TWXku9JI
3+uV0uG7ec/r4QaMLxuzn/SZC/lMN6K+AXTZp9vm9UwZfIOmfPnmObmWP+0HDCBq
xy+MN5G+cI1pW6jALt4IXKsyaQCFbctz8Nux4t+y7JTvKDRZT4fWHuDXpcS2GaXi
HyRI5ZS4jfpJRH0p03PvkEFofVKsZfLAlwnx7HhPqztyc6huu7njjP6XUYbvrhOS
otE/wyyLyT36rQvUVLHfrco2Ib1r58sWb1hL84/5HnbxwsEelBAAGL5VGu2f0zLP
qxmy4XWJg9U/UKzG9gXmbcpDUn1pUcZV1rSSJ/wfbdvBSLF+dQaE6QdjGeAyrt7M
I7gCViECgYEA5ui1aXomHIwqbzll/qotebYNL8AOcpgRJTtzODcnzoe/5aA7yVox
vOFTD9rODaVRhtdC/ZHsU6rID+tx9gQdjLUEXUqS3KsPXSiwbYBPXnEtpgWz/RAu
qpxpMn3kfVbQgJjkfksWvlHNzLj5q4ZhZyL/WV3P9DON8vuP/hwwQLMCgYEAzyiv
rXx/ERnEQOpXEvPtCj04QUGCyW2o4nYIrN3OfO3PM8PnNfB8Z2LKDyuzX9qvWghg
z81KSNByW4OLdvrnX5natfxdu/6qAl18N+z/DFXuTgXi8nG9/2R5c3Gn5CQ5yab7
S4G4Pt7YwfoiqFv9j2b0DE3e5UVicGQIuWmaQ/UCgYBf61swraUXRsJ54YYU++xY
Gt/vbgCSacj1hnSebYsDqDB22tD7G5R9ubwfYe0mjf4H3XPekbdyKgdhVZTJdXww
7yEY/9lyAT0onbZsRliyCqHDzaqu/QHlrYOljdZUmrOSN/Dy5Y5VEPZEjLJqJjBf
/5HDNc0kzCWzQfzWui7xMQKBgGNA8ysEAz2GQul3XdDO3juRqWpaoPcxe0FFnFJ2
04A30JbUveqyFmjShE1QetjqRim06e2mRnksph4CoMeY31KGvKuFBsQT+BC6CdIh
0vFuGod3eoz+wjGjSi1tvysn0Cg1wSEkPcqhqukFl6VirdIPWc6rYKgo3klLJILx
feAhAoGABC0apuKQD2IZZXZtDuUI9I4AemPPh0yKvFfTJxmxQ0fTlWjqdcG5nYdh
tSMBlZwsd6DRlK7dWJ/WHZXuXNeOX6ehSQFmql5/XPNd7INa5My6DDPZr1chh0WJ
QgK94NXJDoDd1OZjpUBMPLVa8d20/RdGNW8OMolJpzEPhg0r7Ac=
-----END RSA PRIVATE KEY-----`

// Confirm the targetURL is valid and can be listened on before running tests.
func init() {
	u, err := url.Parse(targetURL)
	if err != nil {
		panic(fmt.Sprintf("Invalid target URL: %s", err))
	}
	ln, err := net.Listen("tcp", u.Host)
	if err != nil {
		panic(fmt.Sprintf("Unable to listen on address (%s): %s", targetURL, err))
	}
	ln.Close()
}

var _ = Describe("HTTPSDispatcher", func() {
	var (
		dispatcher *mbus.HTTPSDispatcher
		logger     *fakelogger.FakeLogger
	)

	BeforeEach(func() {
		logger = &fakelogger.FakeLogger{}
		serverURL, err := url.Parse(targetURL)
		Expect(err).ToNot(HaveOccurred())
		dispatcher = mbus.NewHTTPSDispatcher(serverURL, settings.CertKeyPair{
			Certificate: agentCert,
			PrivateKey:  agentKey,
			CA:          "",
		}, logger)

		go func() {
			defer GinkgoRecover()
			if err := dispatcher.Start(); err != nil {
				Expect(err).To(MatchError("accept tcp 127.0.0.1:7789: use of closed network connection"))
			}
		}()

		Eventually(func() *http.Response {
			client := getHTTPClient()
			response, _ := client.Get(targetURL + "/example")
			return response
		}, 5*time.Second).ShouldNot(BeNil())
	})

	AfterEach(func() {
		dispatcher.Stop()
	})

	It("calls the handler function for the route", func() {
		var hasBeenCalled = false
		handler := func(w http.ResponseWriter, r *http.Request) {
			hasBeenCalled = true
			w.WriteHeader(201)
		}

		dispatcher.AddRoute("/example", handler)

		client := getHTTPClient()
		response, err := client.Get(targetURL + "/example")

		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(BeNumerically("==", 201))
		Expect(hasBeenCalled).To(Equal(true))
	})

	It("returns a 404 if the route does not exist", func() {
		client := getHTTPClient()
		response, err := client.Get(targetURL + "/example")
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(BeNumerically("==", 404))
	})

	It("does not allow connections using SSLv3", func() {
		handler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
		dispatcher.AddRoute("/example", handler)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionSSL30,
			MaxVersion:         tls.VersionSSL30,
		}
		client := getHTTPClientWithConfig(tlsConfig)
		_, err := client.Get(targetURL + "/example")
		Expect(err).To(HaveOccurred())
	})

	It("does not allow connections using TLSv1", func() {
		handler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
		dispatcher.AddRoute("/example", handler)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS10,
			MaxVersion:         tls.VersionTLS10,
		}
		client := getHTTPClientWithConfig(tlsConfig)
		_, err := client.Get(targetURL + "/example")
		Expect(err).To(HaveOccurred())
	})

	It("does not allow connections using TLSv1.1", func() {
		handler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
		dispatcher.AddRoute("/example", handler)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS11,
			MaxVersion:         tls.VersionTLS11,
		}
		client := getHTTPClientWithConfig(tlsConfig)
		_, err := client.Get(targetURL + "/example")
		Expect(err).To(HaveOccurred())
	})

	It("does allow connections using TLSv1.2", func() {
		handler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
		dispatcher.AddRoute("/example", handler)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS12,
		}
		client := getHTTPClientWithConfig(tlsConfig)
		_, err := client.Get(targetURL + "/example")
		Expect(err).ToNot(HaveOccurred())
	})

	It("does not allow connections using 3DES ciphers", func() {
		handler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
		dispatcher.AddRoute("/example", handler)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			CipherSuites: []uint16{
				tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
			},
		}
		client := getHTTPClientWithConfig(tlsConfig)
		_, err := client.Get(targetURL + "/example")
		Expect(err).To(HaveOccurred())
	})

	It("does not allow connections using RC4 ciphers", func() {
		handler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
		dispatcher.AddRoute("/example", handler)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			CipherSuites: []uint16{
				tls.TLS_RSA_WITH_RC4_128_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
				tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
			},
		}
		client := getHTTPClientWithConfig(tlsConfig)
		_, err := client.Get(targetURL + "/example")
		Expect(err).To(HaveOccurred())
	})

	It("does allow connections using AES ciphers", func() {
		handler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
		dispatcher.AddRoute("/example", handler)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			CipherSuites: []uint16{
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			},
		}
		client := getHTTPClientWithConfig(tlsConfig)
		_, err := client.Get(targetURL + "/example")
		Expect(err).ToNot(HaveOccurred())
	})

	It("logs the request", func() {
		handler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
		dispatcher.AddRoute("/example", handler)
		client := getHTTPClient()
		_, err := client.Get(targetURL + "/example")
		Expect(err).ToNot(HaveOccurred())
		Expect(logger.InfoCallCount()).To(Equal(1))
		tag, message, _ := logger.InfoArgsForCall(0)
		Expect(message).To(Equal("GET /example"))
		Expect(tag).To(Equal("HTTPS Dispatcher"))
	})

	Context("When the basic authorization is wrong", func() {
		It("returns 401", func() {
			dispatcher.AddRoute("/example", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
			})
			client := getHTTPClient()

			response, err := client.Get("https://bad:creds@127.0.0.1:7789/example")

			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(BeNumerically("==", 401))
			Expect(response.Header.Get("WWW-Authenticate")).To(Equal(`Basic realm=""`))
		})
	})
})

func getHTTPClient() http.Client {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		// Both CBC & RC4 ciphers can be exploited
		// Mozilla's "Modern" recommended settings only overlap with the golang TLS client on these two ciphers
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
		MinVersion: tls.VersionTLS12,
	}
	return getHTTPClientWithConfig(tlsConfig)
}

func getHTTPClientWithConfig(tlsConfig *tls.Config) http.Client {
	httpTransport := &http.Transport{TLSClientConfig: tlsConfig}
	return http.Client{Transport: httpTransport}
}
