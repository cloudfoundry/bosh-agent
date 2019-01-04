package blobstore

import (
	utilblobstore "github.com/cloudfoundry/bosh-utils/blobstore"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

const logTag = "cascadingBlobstore"

type cascadingBlobstore struct {
	innerBlobstore utilblobstore.DigestBlobstore
	blobManagers   []BlobManagerInterface
	logger         boshlog.Logger
}

func NewCascadingBlobstore(
	innerBlobstore utilblobstore.DigestBlobstore,
	blobManagers []BlobManagerInterface,
	logger boshlog.Logger,
) utilblobstore.DigestBlobstore {
	return cascadingBlobstore{
		innerBlobstore: innerBlobstore,
		blobManagers:   blobManagers,
		logger:         logger,
	}
}

func (b cascadingBlobstore) Get(blobID string, digest boshcrypto.Digest) (string, error) {
	for _, blobManager := range b.blobManagers {
		if blobManager.BlobExists(blobID) {
			blobPath, err := blobManager.GetPath(blobID, digest)

			if err != nil {
				return "", err
			}

			b.logger.Debug(logTag, "Found blob with BlobManager. BlobID: %s", blobID)
			return blobPath, nil
		}
	}

	return b.innerBlobstore.Get(blobID, digest)
}

func (b cascadingBlobstore) CleanUp(fileName string) error {
	return b.innerBlobstore.CleanUp(fileName)
}

func (b cascadingBlobstore) Create(fileName string) (string, boshcrypto.MultipleDigest, error) {
	return b.innerBlobstore.Create(fileName)
}

func (b cascadingBlobstore) Validate() error {
	return b.innerBlobstore.Validate()
}

func (b cascadingBlobstore) Delete(blobID string) error {
	for _, blobManager := range b.blobManagers {
		if blobManager.BlobExists(blobID) {
			return blobManager.Delete(blobID)
		}
	}

	// We do not delete from the inner blobstore as external blobstores do not
	// currently support deletion. Also, the bosh-agent should not be responsible
	// for deleting blobs from the external blobstore which is managed by the
	// BOSH Director.
	return nil
}
