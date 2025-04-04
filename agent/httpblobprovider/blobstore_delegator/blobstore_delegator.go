package blobstore_delegator //nolint:revive

import (
	"fmt"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"

	"github.com/cloudfoundry/bosh-utils/blobstore"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"

	"github.com/cloudfoundry/bosh-agent/v2/agent/httpblobprovider"
)

type BlobstoreDelegatorImpl struct {
	h      httpblobprovider.HTTPBlobProvider
	b      blobstore.DigestBlobstore
	logger boshlog.Logger
}

func NewBlobstoreDelegator(hp httpblobprovider.HTTPBlobProvider, bp blobstore.DigestBlobstore, logger boshlog.Logger) *BlobstoreDelegatorImpl {
	return &BlobstoreDelegatorImpl{
		h:      hp,
		b:      bp,
		logger: logger,
	}
}

func (b *BlobstoreDelegatorImpl) Get(digest boshcrypto.Digest, signedURL, blobID string, headers map[string]string) (fileName string, err error) {
	if signedURL == "" {
		if blobID == "" {
			return "", fmt.Errorf("Both signedURL and blobID are blank which is invalid") //nolint:staticcheck
		}
		return b.b.Get(blobID, digest)
	}

	getBlobRetryable := boshretry.NewRetryable(func() (bool, error) {
		fileName, err = b.h.Get(signedURL, digest, headers)
		if err != nil {
			return true, bosherr.WrapError(err, "Failed to download blob")
		}
		return false, nil
	})

	attemptRetryStrategy := boshretry.NewAttemptRetryStrategy(3, 5*time.Second, getBlobRetryable, b.logger)
	err = attemptRetryStrategy.Try()
	if err != nil {
		return "", err
	}

	return fileName, nil
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
