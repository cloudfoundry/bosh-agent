package httpblobprovider

import (
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

//go:generate counterfeiter . HTTPBlobProvider

type HTTPBlobProvider interface {
	Upload(signedURL, filepath string, headers map[string]string) (boshcrypto.MultipleDigest, error)
	Get(signedURL string, digest boshcrypto.Digest, headers map[string]string) (string, error)
}
