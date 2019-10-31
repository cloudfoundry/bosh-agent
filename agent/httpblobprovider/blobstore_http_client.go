package httpblobprovider

import (
	"crypto/x509"
	"net/http"

	"github.com/cloudfoundry/bosh-agent/settings"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	boshhttp "github.com/cloudfoundry/bosh-utils/httpclient"
)

func NewBlobstoreHTTPClient(blobstoreSettings settings.Blobstore) (*http.Client, error) {
	var certpool *x509.CertPool

	caCert := fetchCaCertificate(blobstoreSettings.Options)
	if caCert != "" {
		var err error

		certpool, err = boshcrypto.CertPoolFromPEM([]byte(caCert))
		if err != nil {
			return nil, err
		}
	}

	return boshhttp.CreateDefaultClient(certpool), nil
}

func fetchCaCertificate(options map[string]interface{}) string {
	if options == nil {
		return ""
	}

	tls, ok := options["tls"]
	if !ok {
		return ""
	}

	tlsMap, ok := tls.(map[string]interface{})
	if !ok {
		return ""
	}

	cert, ok := tlsMap["cert"]
	if !ok {
		return ""
	}

	certMap, ok := cert.(map[string]interface{})
	if !ok {
		return ""
	}

	ca, ok := certMap["ca"].(string)
	if !ok {
		return ""
	}

	return ca
}
