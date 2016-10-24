package blobstore

import (
	boshUtilsBlobStore "github.com/cloudfoundry/bosh-utils/blobstore"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"path/filepath"
)

type cascadingBlobstore struct {
	fs        boshsys.FileSystem
	blobstore boshUtilsBlobStore.Blobstore
	blobsDir  string
}

func NewCascadingBlobstore(fs boshsys.FileSystem, blobstore boshUtilsBlobStore.Blobstore, blobsDir string) boshUtilsBlobStore.Blobstore {
	return cascadingBlobstore{
		fs:        fs,
		blobstore: blobstore,
		blobsDir:  blobsDir,
	}
}

func (b cascadingBlobstore) Get(blobID, fingerprint string) (string, error) {
	localBlobPath := filepath.Join(b.blobsDir, blobID)

	if b.fs.FileExists(localBlobPath) {
		return b.copyToTmpFile(localBlobPath)
	}

	return b.blobstore.Get(blobID, fingerprint)
}

func (b cascadingBlobstore) CleanUp(fileName string) error {
	return b.blobstore.CleanUp(fileName)
}

func (b cascadingBlobstore) Create(fileName string) (string, string, error) {
	return b.blobstore.Create(fileName)
}

func (b cascadingBlobstore) Validate() error {
	return b.blobstore.Validate()
}

func (b cascadingBlobstore) Delete(blobID string) error {
	localBlobPath := filepath.Join(b.blobsDir, blobID)

	if b.fs.FileExists(localBlobPath) {
		b.fs.RemoveAll(localBlobPath)
	}

	return b.blobstore.Delete(blobID)
}

func (b cascadingBlobstore) cleanUploadedFile() error {

	return nil
}

func (b cascadingBlobstore) copyToTmpFile(srcFileName string) (string, error) {
	file, err := b.fs.TempFile("bosh-blobstore-cascading-Get")
	if err != nil {
		return "", bosherr.WrapError(err, "Creating temporary file")
	}

	destTmpFileName := file.Name()

	err = b.fs.CopyFile(srcFileName, destTmpFileName)
	if err != nil {
		b.fs.RemoveAll(destTmpFileName)
		return "", bosherr.WrapError(err, "Copying file")
	}

	return destTmpFileName, nil
}
