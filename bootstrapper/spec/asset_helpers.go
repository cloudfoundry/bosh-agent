package spec

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"

	. "github.com/onsi/gomega"
)

var specDir string

func init() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("WAT")
	}
	specDir = filepath.Dir(file)
}

func FixtureFilename(name string) string {
	return filepath.Join(specDir, "assets", name)
}

func FixtureData(name string) []byte {
	bytes, err := ioutil.ReadFile(FixtureFilename(name))
	Expect(err).ToNot(HaveOccurred())
	return bytes
}

func CertFor(certName string) *tls.Certificate {
	certFile := FixtureFilename(fmt.Sprintf("certs/%s.crt", certName))
	keyFile := FixtureFilename(fmt.Sprintf("certs/%s.key", certName))
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	Expect(err).ToNot(HaveOccurred())
	return &cert
}

func CertPool() *x509.CertPool {
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(([]byte)(FixtureData("certs/rootCA.pem"))) {
		fmt.Println("Wha? cert failed")
		Expect(true).To(Equal(false))
	}
	return certPool
}
