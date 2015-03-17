package kickstarter_test

import (
	. "github.com/onsi/gomega"

	"crypto/tls"
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

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	if clientCert != nil {
		tr.TLSClientConfig.Certificates = []tls.Certificate{*clientCert}
	}

	client := &http.Client{Transport: tr}
	return client.Do(req)
}

//func publicKey(priv interface{}) interface{} {
//	switch k := priv.(type) {
//	case *rsa.PrivateKey:
//		return &k.PublicKey
//	case *ecdsa.PrivateKey:
//		return &k.PublicKey
//	default:
//		return nil
//	}
//}
//
//func pemBlockForKey(priv interface{}) *pem.Block {
//	switch k := priv.(type) {
//	case *rsa.PrivateKey:
//		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
//	case *ecdsa.PrivateKey:
//		b, err := x509.MarshalECPrivateKey(k)
//		if err != nil {
//			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
//			os.Exit(2)
//		}
//		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
//	default:
//		return nil
//	}
//}
//
//func generateCert(tmpDir string) (certFile string, keyFile string, err error) {
//	priv, err := rsa.GenerateKey(rand.Reader, 2048)
//	if err != nil {
//		log.Fatalf("failed to generate private key: %s", err)
//	}
//
//	notBefore := time.Now().Add(-1)
//	notAfter := notBefore.Add(1000)
//
//	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
//	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
//	if err != nil {
//		log.Fatalf("failed to generate serial number: %s", err)
//	}
//
//	template := x509.Certificate{
//		SerialNumber: serialNumber,
//		Subject: pkix.Name{
//			Organization: []string{"Acme Co"},
//		},
//		NotBefore:             notBefore,
//		NotAfter:              notAfter,
//		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
//		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
//		BasicConstraintsValid: true,
//	}
//
//	template.DNSNames = append(template.DNSNames, "localhost")
//
//	isCA := true
//	if isCA {
//		template.IsCA = true
//		template.KeyUsage |= x509.KeyUsageCertSign
//	}
//
//	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
//	if err != nil {
//		log.Fatalf("Failed to create certificate: %s", err)
//	}
//
//	certOut, err := os.Create(path.Join(tmpDir, "cert.pem"))
//	if err != nil {
//		log.Fatalf("failed to open cert.pem for writing: %s", err)
//		return
//	}
//	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
//	certOut.Close()
//	log.Print("written cert.pem\n")
//
//	keyOut, err := os.OpenFile(path.Join(tmpDir, "key.pem"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
//	if err != nil {
//		log.Print("failed to open key.pem for writing:", err)
//		return
//	}
//	pem.Encode(keyOut, pemBlockForKey(priv))
//	keyOut.Close()
//	log.Print("written key.pem\n")
//	certFile = certOut.Name()
//	keyFile = keyOut.Name()
//	err = nil
//	return
//}
