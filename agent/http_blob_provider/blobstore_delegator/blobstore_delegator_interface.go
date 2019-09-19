package blobstore_delegator

import (
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

//go:generate counterfeiter . BlobstoreDelegator

type BlobstoreDelegator interface {
	Get(digest boshcrypto.Digest, signedURL, blobID string) (fileName string, err error)
	Write(signedURL, path string) (string, boshcrypto.MultipleDigest, error)
	CleanUp(signedURL, path string) error
	Delete(signedURL, blobID string) error
}
