package mbus

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
)

func TestNATSPeerNames(t *testing.T) {
	handler := &natsHandler{}
	for _, name := range []string{"director.nats.bosh-internal", "*.nats.bosh-internal"} {
		cert := &x509.Certificate{Subject: pkix.Name{CommonName: name}}
		if err := handler.VerifyPeerCertificate(nil, [][]*x509.Certificate{{cert}}); err != nil {
			t.Fatalf("valid peer %q: %v", name, err)
		}
	}
	for _, name := range []string{"xnats.bosh-internal", "xnatsxbosh-internal", "director-nats-bosh-internal", ".nats.bosh-internal", "director.nats.bosh-internal.evil"} {
		cert := &x509.Certificate{Subject: pkix.Name{CommonName: name}}
		if err := handler.VerifyPeerCertificate(nil, [][]*x509.Certificate{{cert}}); err == nil {
			t.Fatalf("accepted peer %q", name)
		}
	}
}
