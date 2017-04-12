package yagnats

import (
	"crypto/x509"
	. "gopkg.in/check.v1"
	"os/exec"
)

type TLSSuite struct {
	Client   *Client
	NatsConn NATSConn
	NatsCmd  *exec.Cmd
}

var CA = `-----BEGIN CERTIFICATE-----
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
+Svp
-----END CERTIFICATE-----`

var InvalidCA = `-----BEGIN CERTIFICATE-----
MIIDFDCCAfygAwIBAgIRANn247vhGXLev3Ltw8NOIQAwDQYJKoZIhvcNAQELBQAw
MzEMMAoGA1UEBhMDVVNBMRYwFAYDVQQKEw1DbG91ZCBGb3VuZHJ5MQswCQYDVQQD
EwJjYTAeFw0xNzA0MDMxOTQyMTVaFw0xODA0MDMxOTQyMTVaMDMxDDAKBgNVBAYT
A1VTQTEWMBQGA1UEChMNQ2xvdWQgRm91bmRyeTELMAkGA1UEAxMCY2EwggEiMA0G
CSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCpcve+8iQmzj2/dUfGpI55CmZ7aYAr
CsXsB6ztceewhMKTOo1pgBYT5T3G759Leab4id2JjxJB2sjou7g69pCTWNFkKz0G
ED2RMGfuMACfISezE5fhSKdNR0vyleSEgvwOcdWa0PP6pTK//iD7p4fyx5HigpWt
7hxmUTsqzOBOOYv1tw7ZhX6msZ5EL4d58rIbqozz8Hr/5mw/izUr2w0dCuuXTb8k
qIrh1PjPwBoOW38yXZ/Pyex14NQMiqVqH2gMSwXpZNdVi9whVGrzP3ZAUv5uyICK
j4KGBFJ+NcFq9VI2lbBUNdCD4MqdzaSA7OSnhaYYku2KUwIlBG9CtQctAgMBAAGj
IzAhMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEB
CwUAA4IBAQBLcq6xTPGYhA5Blhbkja3kd7AsWsVOv/HvLnxUJLwY8SLDhDHVndRR
NvNmmXQOMDZ9tcLUl4Jgoy+u2XnQxTfpvPwT0qX958spcwCo9mQJKuOFcZfNwS8M
bSTo1k+a33YtB8AWyS0GabG+2PEp/ARptJiQ6OMDKDLFMKK4NqpSl8cXNmPf5bEO
67qHgr+2xtS4Mkj+EhZJuVpqIU3jL7psIQWdEm7dAy+qmZaB44LT1AMcUINgBsor
bew6/PW7wNhEW/GWI/Nvef3EsFh80bYHq21eW6RdaSLgwddcmi6ak4CxizPYK57e
XtrIuun84K30EXBrBdtUqWBwgBtu/HT2
-----END CERTIFICATE-----`

var _ = Suite(&TLSSuite{})

func (t *TLSSuite) SetUpSuite(c *C) {
	t.NatsCmd = startNatsTLS(4555)
	waitUntilNatsUp(4555)
}

func (t *TLSSuite) TearDownSuite(c *C) {
	stopCmd(t.NatsCmd)
}

func (t *TLSSuite) TestNewTLSConnection(c *C) {
	client := NewClient()

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM([]byte(CA))
	c.Assert(ok, Equals, true)

	err := client.Connect(&ConnectionInfo{Addr: "127.0.0.1:4555",
		Username: "nats",
		Password: "nats",
		CertPool: roots,
	})
	c.Assert(err, IsNil)
	t.Client = client

	pingSuccess := client.Ping()
	c.Assert(pingSuccess, Equals, true)
}

func (t *TLSSuite) TestNewTLSConnectionWithWrongCA(c *C) {
	client := NewClient()

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM([]byte(InvalidCA))
	c.Assert(ok, Equals, true)

	err := client.Connect(&ConnectionInfo{Addr: "127.0.0.1:4555",
		Username: "nats",
		Password: "nats",
		CertPool: roots,
	})

	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "x509: certificate signed by unknown authority")

}

func (t *TLSSuite) TestNewTLSConnectionWithEmptyCertPool(c *C) {
	client := NewClient()

	err := client.Connect(&ConnectionInfo{Addr: "127.0.0.1:4555",
		Username: "nats",
		Password: "nats",
		CertPool: x509.NewCertPool(),
	})

	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "x509: certificate signed by unknown authority")

}
