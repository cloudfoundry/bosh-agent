package httpblobprovider_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	httpblobprovider "github.com/cloudfoundry/bosh-agent/v2/agent/httpblobprovider"
	"github.com/cloudfoundry/bosh-agent/v2/settings"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

var _ = Describe("NewBlobstoreHTTPClient", func() {
	var (
		options     settings.Blobstore
		certificate = `-----BEGIN CERTIFICATE-----
MIIEOjCCAqKgAwIBAgIRALcJj7JFUaQ2sd4gwuIqILcwDQYJKoZIhvcNAQELBQAw
JjEMMAoGA1UEBhMDVVNBMRYwFAYDVQQKEw1DbG91ZCBGb3VuZHJ5MB4XDTIxMDky
OTE0MzM0NVoXDTIyMDkyOTE0MzM0NVowJjEMMAoGA1UEBhMDVVNBMRYwFAYDVQQK
Ew1DbG91ZCBGb3VuZHJ5MIIBojANBgkqhkiG9w0BAQEFAAOCAY8AMIIBigKCAYEA
sjOL71UfPTP9GQKSRX9xp+eKA3rB/5Sfv4jfiQABwComj+lvUxloZq8pBrs0UFje
kjsaiLDhvYPkNyXu4ATKh8kaA558MJ5+3KChOi9+BY9xzFp+suY95sVU0R1jom+U
WEZetmk8CTfClxTbvbu1wSjZA4S966prf/5NIw60SKyo4uSSQ/CfEnzzFCrrbEjX
i2IIELTuhxc2J7neFQb0bMZkMn/GKqi2/IZusVl7ZYip0ntCilTSKiDSzp7I5YEq
rFheaLnyiID0Aig/+ndcYesVx08ECHss6Q7SF6eS8BFovZuyj//loaKuNW3jVwZE
/MS46KIU6kQxKQB+blLyDlXSS7ZtmFICYV8a51D2vdBKAb3u344Hf1zuJ8bY5yf2
+BnNcksLURn7iuQiMBbPVe16mU+jZUKrLF6Wuz3e7+7Nl3vowzm3h2GYOle0jeh6
EeeY9ZXlFpDjLQW3zbZ/4z18vJITbfaqK2RaOOja44HnuZnVtMcJlPzvsf6PlwSl
AgMBAAGjYzBhMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1Ud
DgQWBBSZqhw0Sa+fSwNLp1TueSsiAI/I4jAfBgNVHSMEGDAWgBSZqhw0Sa+fSwNL
p1TueSsiAI/I4jANBgkqhkiG9w0BAQsFAAOCAYEAMyvKR6JniyS2vY4DX4LetkS0
/Z5nIYsbqLiR7wJl27aCI8h0RyKJTRYix5E5cxY/grYtw5TAaSqwr8yVGV+mvIr3
p7Z27yB4k2P4IiAt/4gwdcvd2ng1cqhON5vpJRPXDPEjBH/Szb1EfixM01PGU4Q1
XIIff4AMcJf8lZQQZEaWcIH8WMZsbm9D2Ro0d7FXxOfiO/tc90SBB/4Eq5NBvH6X
oJsIY8mqk0IR5bXtRPDoOgL9nrIzXB9IbNlGwqLfcV0ouHIXtSdsOO5QQx4cW8Mv
Aios59acWsSCk2LG5SuRkVR7zkMZmDFiJ9zLRLC+TxjRRBAKTAE8fXeHqHVFcWMK
fJo3Mli9ryp1/HRECrSkEDE5zK2SsGbtuciYhA4ALVQB02EQSlhRYL9zXtfnq6d5
aMaUJX87g3h6U88UGBrIQZE9frYi97wpasdJqrZ8XuROhws3xznrPlM0Debv69/h
Ia61hX5ccGHhfw1oXlPCqF8jPZgVJpSAv5nsthy9
-----END CERTIFICATE-----`
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

			Expect(client.Transport.(*http.Transport).TLSClientConfig.RootCAs.Subjects()).To(Equal(expectedCertPool.Subjects())) //nolint:staticcheck
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
