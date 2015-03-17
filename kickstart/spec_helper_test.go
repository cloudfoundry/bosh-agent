package kickstart_test

import (
	. "github.com/onsi/gomega"

	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	http "net/http"
	os "os"
	"regexp"
)

type mutableWriter struct {
	out      io.Writer
	patterns []*regexp.Regexp
	captured bytes.Buffer
}

func (mw *mutableWriter) Write(p []byte) (n int, err error) {
	for _, pattern := range mw.patterns {
		if pattern.Match(p) {
			return mw.captured.Write(p)
		}
	}

	n, err = mw.out.Write(p)
	return
}

func (mw *mutableWriter) Capture(pattern string) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	mw.patterns = append(mw.patterns, re)
}

func (mw *mutableWriter) Captured() string {
	return mw.captured.String()
}

func fileExists(name string) bool {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return false
	}
	return true
}

func fixtureData(name string) []byte {
	bytes, err := ioutil.ReadFile(fixtureFilename(name))
	Expect(err).ToNot(HaveOccurred())
	return bytes
}

func fixtureFilename(name string) string {
	return fmt.Sprintf("spec/support/%s", name)
}

func getFreePort() int {
	listener, err := net.ListenTCP("tcp", nil)
	Expect(err).ToNot(HaveOccurred())

	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func certFor(certName string) *tls.Certificate {
	certFile := fixtureFilename(fmt.Sprintf("certs/%s.crt", certName))
	keyFile := fixtureFilename(fmt.Sprintf("certs/%s.key", certName))
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	Expect(err).ToNot(HaveOccurred())
	return &cert
}

func httpClient(clientCert *tls.Certificate) *http.Client {
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(([]byte)(fixtureData("certs/rootCA.pem"))) {
		fmt.Println("Wha? cert failed")
		Expect(true).To(Equal(false))
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: certPool,
		},
	}

	if clientCert != nil {
		tr.TLSClientConfig.Certificates = []tls.Certificate{*clientCert}
	}

	return &http.Client{Transport: tr}
}

func httpDo(method, url string, clientCert *tls.Certificate) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	Expect(err).ToNot(HaveOccurred())
	return httpClient(clientCert).Do(req)
}

func httpPut(url, uploadFile string, clientCert *tls.Certificate) (*http.Response, error) {
	reader, err := os.Open(uploadFile)
	Expect(err).ToNot(HaveOccurred())
	req, err := http.NewRequest("PUT", url, reader)
	Expect(err).ToNot(HaveOccurred())
	return httpClient(clientCert).Do(req)
}
