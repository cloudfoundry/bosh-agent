package kickstarter_test

import (
	. "github.com/onsi/gomega"

	"crypto/tls"
	"net"
	http "net/http"
	"os"
)

func getFreePort() int {
	listener, err := net.ListenTCP("tcp", nil)
	Expect(err).ToNot(HaveOccurred())

	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func httpPut(url, file string) (*http.Response, error) {
	reader, err := os.Open(file)
	Expect(err).ToNot(HaveOccurred())
	req, err := http.NewRequest("PUT", url, reader)
	Expect(err).ToNot(HaveOccurred())

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	return client.Do(req)
}
