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

func (b digestVerifiableBlobstore) Get(blobID string, digest boshcrypto.Digest) (string, error) {
	fileName, err := b.blobstore.Get(blobID, digest)
	if err != nil {
		return "", bosherr.WrapError(err, "Getting blob from inner blobstore")
	}


	file, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer file.Close()


	fileDigest, err := digest.Algorithm().CreateDigest(file)
	fileDigest.Verify(digest)

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
