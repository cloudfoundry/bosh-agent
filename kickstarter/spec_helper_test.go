package kickstarter_test

import (
	. "github.com/onsi/gomega"

	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	http "net/http"
	os "os"
)

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

func httpPut(url, uploadFile string, clientCert *tls.Certificate) (*http.Response, error) {
	reader, err := os.Open(uploadFile)
	Expect(err).ToNot(HaveOccurred())
	req, err := http.NewRequest("PUT", url, reader)
	Expect(err).ToNot(HaveOccurred())

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(([]byte)(fixtureData("certs/rootCA.pem"))) {
		fmt.Println("Wha? cert failed")
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: certPool,
		},
	}

	if clientCert != nil {
		tr.TLSClientConfig.Certificates = []tls.Certificate{*clientCert}
	}

	client := &http.Client{Transport: tr}
	return client.Do(req)
}
