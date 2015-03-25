package spec

import (
	"crypto/tls"
	"net/http"
	"os"

	. "github.com/onsi/gomega"
)

func HttpClient(clientCert *tls.Certificate) *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: CertPool(),
		},
	}

	if clientCert != nil {
		tr.TLSClientConfig.Certificates = []tls.Certificate{*clientCert}
	}

	return &http.Client{Transport: tr}
}

func HttpDo(method, url string, clientCert *tls.Certificate) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	Expect(err).ToNot(HaveOccurred())
	return HttpClient(clientCert).Do(req)
}

func HttpPut(url, uploadFile string, clientCert *tls.Certificate) (*http.Response, error) {
	reader, err := os.Open(uploadFile)
	Expect(err).ToNot(HaveOccurred())
	req, err := http.NewRequest("PUT", url, reader)
	Expect(err).ToNot(HaveOccurred())
	return HttpClient(clientCert).Do(req)
}
