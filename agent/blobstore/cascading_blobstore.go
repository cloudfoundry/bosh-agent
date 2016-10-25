package blobstore

import (
	boshUtilsBlobStore "github.com/cloudfoundry/bosh-utils/blobstore"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

const logTag = "cascadingBlobstore"

type cascadingBlobstore struct {
	innerBlobstore boshUtilsBlobStore.Blobstore
	blobManager    boshUtilsBlobStore.BlobManagerInterface
	logger         boshlog.Logger
}

func NewCascadingBlobstore(
	innerBlobstore boshUtilsBlobStore.Blobstore,
	blobManager boshUtilsBlobStore.BlobManagerInterface,
	logger boshlog.Logger) boshUtilsBlobStore.Blobstore {
	return cascadingBlobstore{
		innerBlobstore: innerBlobstore,
		blobManager:    blobManager,
		logger:         logger,
	}
}

func (b cascadingBlobstore) Get(blobID, fingerprint string) (string, error) {
	blobPath, err := b.blobManager.GetPath(blobID)

	if err == nil {
		b.logger.Debug(logTag, "Found blob with BlobManager. BlobID: %s", blobID)
		return blobPath, nil
	}

	return b.innerBlobstore.Get(blobID, fingerprint)
}

func (b cascadingBlobstore) CleanUp(fileName string) error {
	return b.innerBlobstore.CleanUp(fileName)
}

func (b cascadingBlobstore) Create(fileName string) (string, string, error) {
	return b.innerBlobstore.Create(fileName)
}

func (b cascadingBlobstore) Validate() error {
	return b.innerBlobstore.Validate()
}

func (b cascadingBlobstore) Delete(blobID string) error {
	err := b.blobManager.Delete(blobID)

	if err != nil {
		return err
	}

	return b.innerBlobstore.Delete(blobID)
}
