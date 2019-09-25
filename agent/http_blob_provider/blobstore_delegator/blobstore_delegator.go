package blobstore_delegator

import (
	"fmt"

	httpblobprovider "github.com/cloudfoundry/bosh-agent/agent/http_blob_provider"
	"github.com/cloudfoundry/bosh-utils/blobstore"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

type blobstoreDelegator struct {
	h httpblobprovider.HTTPBlobProvider
	b blobstore.DigestBlobstore
}

func NewBlobstoreDelegator(hp httpblobprovider.HTTPBlobProvider, bp blobstore.DigestBlobstore) *blobstoreDelegator {
	return &blobstoreDelegator{
		h: hp,
		b: bp,
	}
}

func (b *blobstoreDelegator) Get(digest boshcrypto.Digest, signedURL, blobID string) (fileName string, err error) {
	if signedURL == "" {
		if blobID == "" {
			return "", fmt.Errorf("Both signedURL and blobID are blank which is invalid")
		}
		return b.b.Get(blobID, digest)
	}
	return b.h.Get(signedURL, digest)
}

func (b *blobstoreDelegator) Write(signedURL, path string) (string, boshcrypto.MultipleDigest, error) {
	if signedURL == "" {
		return b.b.Create(path)
	}

	digest, err := b.h.Upload(signedURL, path)
	return "", digest, err
}

func (b *blobstoreDelegator) CleanUp(signedURL, fileName string) (err error) {
	if signedURL != "" {
		return fmt.Errorf("CleanUp is not supported for signed URLs")
	}
	return b.b.CleanUp(fileName)
}

func (b *blobstoreDelegator) Delete(signedURL, blobID string) (err error) {
	if signedURL != "" {
		return fmt.Errorf("Delete is not supported for signed URLs")
	}
	return b.b.Delete(blobID)
}
