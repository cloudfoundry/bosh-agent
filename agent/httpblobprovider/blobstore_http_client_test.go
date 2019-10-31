package httpblobprovider_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	httpblobprovider "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider"
	"github.com/cloudfoundry/bosh-agent/settings"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

var certificate = `-----BEGIN CERTIFICATE-----
MIIEOTCCAqGgAwIBAgIQUz4Ld6870GCQG6PdYJPcEDANBgkqhkiG9w0BAQsFADAm
MQwwCgYDVQQGEwNVU0ExFjAUBgNVBAoTDUNsb3VkIEZvdW5kcnkwHhcNMTkwOTEx
MjExMDU2WhcNMjAwOTEwMjExMDU2WjAmMQwwCgYDVQQGEwNVU0ExFjAUBgNVBAoT
DUNsb3VkIEZvdW5kcnkwggGiMA0GCSqGSIb3DQEBAQUAA4IBjwAwggGKAoIBgQDP
mWjuhRQsTHaNx9iJrrFCyN6qlIB0+AaNQvQcwm2FU3DGqaFNQakpr0NGZRwVhQAj
sg/RZY6ucYHcuZwum7KpQn7a4BqllSrViCO27OO8z+71T6hbCPWySDV347NH81L4
6xykKWVs82kipkl/ZmUD7D/ntEWLmodxgQ7ddWQxPsY78mSMiszF+KJY22w+wIDM
t8Rv4+NDthT985b3UImJtO3u9OscbtN9vqGOTTB7ALGe5NW1aDlnIk/sdRbGffJd
Rj0u8QHuDZVWqI9t4M+o1/gi6S03EyyV3na0vTmIfJBUaB0I5tshOUQFgP6T7wQ+
cfFMpRIDpN4udj06PD16LWd0USUyc8Cpd554WhodjDcalxEQ5VPXFSbfgBsuqVKy
LcsH5FZtO3+CtKSNnDqVbF/qZEdOZASecSCmNGI3pwpM8bZ7MwdJUA5BJAoK5Ooe
5CMClG+M5dKykLfpGXI8RcvwbeHA4745XZW2XQ+b+JsS45Yxg95Zdf29a5fDiy0C
AwEAAaNjMGEwDgYDVR0PAQH/BAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0O
BBYEFOJMTmyL9XnRz7iHwunx9EaIK9fTMB8GA1UdIwQYMBaAFOJMTmyL9XnRz7iH
wunx9EaIK9fTMA0GCSqGSIb3DQEBCwUAA4IBgQCYaKHnwJJKCqll43ppFVmcjyyL
yj89PcE2ibaYlacd7qNhS4knXFQENRXC4Oaw8WWpmB2lwU47XRBqEeg9HFUQAWNb
/5B/XeVlYm2xy09N/RgL8u0Ahmt38+UsAc0PFoYbqVPPE/O7FTeLqescQvNT5iDt
NRKVRlniJ2N+mNCylxkj/pIwm1FCiVTBerV1bjW6pUL7MlFxcb1Nvrfkg4eeeJ7H
gbK6FlRpc363x//YhgfCcaqrX2vipc8bn+8MRAZyoiXKp/IRJkMfdM6KWI+WytD1
qF9YZne4mM1T0AQJH08v+fmP8sbQh050WnxaUIcfxKbBtqnMxWz8pKHI9uy13Abz
R1nwBKbLG8u5UziY6tKh1AltHGZddXSmIf6yPCB5DtA8hXYz+Y0cfmetgJYZb/l2
joMIjVkEt4lJZEF/BLB8qMGcsIby78EAanR2Qwvuiu2dvn5PAPmYwER2o4gp0dR0
Y3agqP0Bdgn7WOUW56e5d3m8X4xu4JZJBCwFqzc=
-----END CERTIFICATE-----`

var _ = Describe("NewBlobstoreHTTPClient", func() {
	var (
		options settings.Blobstore
	)

	BeforeEach(func() {
		options = settings.Blobstore{
			Type: "dav",
		}
	})

	Context("when the ca certificate is defined in the blobstore configuration", func() {
		BeforeEach(func() {
			options = settings.Blobstore{
				Type: "dav",
				Options: map[string]interface{}{
					"tls": map[string]interface{}{
						"cert": map[string]interface{}{
							"ca": certificate,
						},
					},
				},
			}
		})

		It("parses the ca certificate and constructs the client", func() {
			client, err := httpblobprovider.NewBlobstoreHTTPClient(options)
			Expect(err).NotTo(HaveOccurred())

			expectedCertPool, err := boshcrypto.CertPoolFromPEM([]byte(certificate))
			Expect(err).NotTo(HaveOccurred())

			Expect(client.Transport.(*http.Transport).TLSClientConfig.RootCAs).To(Equal(expectedCertPool))
		})

		Context("when the ca certificate is not valid", func() {
			BeforeEach(func() {
				options = settings.Blobstore{
					Type: "dav",
					Options: map[string]interface{}{
						"tls": map[string]interface{}{
							"cert": map[string]interface{}{
								"ca": "invalid-ca",
							},
						},
					},
				}
			})

			It("returns an error", func() {
				_, err := httpblobprovider.NewBlobstoreHTTPClient(options)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("when the ca certificate is not defined in the blobstore configuration", func() {
		It("constructs an http client", func() {
			client, err := httpblobprovider.NewBlobstoreHTTPClient(options)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Transport.(*http.Transport).TLSClientConfig.RootCAs).To(BeNil())
		})
	})
})
