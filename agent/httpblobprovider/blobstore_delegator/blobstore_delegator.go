package blobstore_delegator

import (
	"fmt"

	httpblobprovider "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider"
	"github.com/cloudfoundry/bosh-utils/blobstore"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

type BlobstoreDelegatorImpl struct {
	h httpblobprovider.HTTPBlobProvider
	b blobstore.DigestBlobstore
}

func NewBlobstoreDelegator(hp httpblobprovider.HTTPBlobProvider, bp blobstore.DigestBlobstore) *BlobstoreDelegatorImpl {
	return &BlobstoreDelegatorImpl{
		h: hp,
		b: bp,
	}
}

func (b *BlobstoreDelegatorImpl) Get(digest boshcrypto.Digest, signedURL, blobID string, headers map[string]string) (fileName string, err error) {
	if signedURL == "" {
		if blobID == "" {
			return "", fmt.Errorf("Both signedURL and blobID are blank which is invalid")
		}
		return b.b.Get(blobID, digest)
	}
	return b.h.Get(signedURL, digest, headers)
}

func (b *BlobstoreDelegatorImpl) Write(signedURL, path string, headers map[string]string) (string, boshcrypto.MultipleDigest, error) {
	if signedURL == "" {
		return b.b.Create(path)
	}

	digest, err := b.h.Upload(signedURL, path, headers)
	return "", digest, err
}

func (b *BlobstoreDelegatorImpl) CleanUp(signedURL, fileName string) (err error) {
	if signedURL != "" {
		return fmt.Errorf("CleanUp is not supported for signed URLs")
	}
	return b.b.CleanUp(fileName)
}

func (b *BlobstoreDelegatorImpl) Delete(signedURL, blobID string) (err error) {
	if signedURL != "" {
		return fmt.Errorf("Delete is not supported for signed URLs")
	}
	return b.b.Delete(blobID)
}
