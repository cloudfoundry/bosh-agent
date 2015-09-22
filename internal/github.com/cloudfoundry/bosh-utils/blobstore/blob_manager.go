package blobstore

import (
	"os"
	"path/filepath"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
)

type BlobManager struct {
	fs            boshsys.FileSystem
	blobstorePath string
}

func NewBlobManager(fs boshsys.FileSystem, blobstorePath string) (manager BlobManager) {
	manager.fs = fs
	manager.blobstorePath = blobstorePath
	return
}

func (manager BlobManager) Fetch(blobID string) (readOnlyFile boshsys.File, err error) {
	blobPath := filepath.Join(manager.blobstorePath, blobID)

	readOnlyFile, err = manager.fs.OpenFile(blobPath, os.O_RDONLY, os.ModeDir)
	if err != nil {
		err = bosherr.WrapError(err, "Reading blob")
	}
	return
}

func (manager BlobManager) Write(blobID string, blobBytes []byte) (err error) {
	blobPath := filepath.Join(manager.blobstorePath, blobID)

	err = manager.fs.WriteFile(blobPath, blobBytes)
	if err != nil {
		err = bosherr.WrapError(err, "Updating blob")
	}
	return
}
