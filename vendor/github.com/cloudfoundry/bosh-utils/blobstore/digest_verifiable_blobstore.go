package blobstore

import (
	"fmt"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"os"
)

type digestVerifiableBlobstore struct {
	blobstore      Blobstore
	digestProvider boshcrypto.DigestProvider
}

func NewDigestVerifiableBlobstore(blobstore Blobstore, digestProvider boshcrypto.DigestProvider) Blobstore {
	return digestVerifiableBlobstore{
		blobstore:      blobstore,
		digestProvider: digestProvider,
	}
}

func (b digestVerifiableBlobstore) Get(blobID string, multiDigest boshcrypto.MultipleDigest) (string, error) {
	fileName, err := b.blobstore.Get(blobID, multiDigest)
	if err != nil {
		return "", bosherr.WrapError(err, "Getting blob from inner blobstore")
	}

	strongestDigest, err := boshcrypto.PreferredDigest(multiDigest)
	if err != nil {
		return "", err
	}

	file, err := os.Open(fileName)
	if err != nil {
		return "", err
	}

	actualDigest, err := b.digestProvider.CreateFromStream(file, strongestDigest.Algorithm())
	if err != nil {
		return "", err
	}

	err = strongestDigest.Verify(actualDigest)
	if err != nil {
		return "", bosherr.WrapError(err, fmt.Sprintf(`Checking downloaded blob "%s"`, blobID))
	}

	return fileName, nil
}

func (b digestVerifiableBlobstore) Delete(blobId string) error {
	return b.blobstore.Delete(blobId)
}

func (b digestVerifiableBlobstore) CleanUp(fileName string) error {
	return b.blobstore.CleanUp(fileName)
}

func (b digestVerifiableBlobstore) Create(fileName string) (string, error) {
	blobID, err := b.blobstore.Create(fileName)
	return blobID, err
}

func (b digestVerifiableBlobstore) Validate() error {
	return b.blobstore.Validate()
}
