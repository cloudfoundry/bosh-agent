package fakes

import (
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

type FakeBlobstore struct {
	GetBlobIDs      []string
	GetFingerprints []boshcrypto.Digest
	GetFileName     string
	GetFileNames    []string
	GetError        error
	GetErrs         []error

	CleanUpFileName string
	CleanUpErr      error

	DeleteBlobID string
	DeleteErr    error

	CreateFileNames    []string
	CreateBlobID       string
	CreateBlobIDs      []string
	CreateErr          error
	CreateErrs         []error
	CreateCallBack     func()

	ValidateError error
}

func NewFakeBlobstore() *FakeBlobstore {
	return &FakeBlobstore{}
}

func (bs *FakeBlobstore) Get(blobID string, fingerprint boshcrypto.Digest) (string, error) {
	bs.GetBlobIDs = append(bs.GetBlobIDs, blobID)
	bs.GetFingerprints = append(bs.GetFingerprints, fingerprint)

	fileName, err := bs.GetFileName, bs.GetError

	if len(bs.GetFileNames) > 0 {
		fileName = bs.GetFileNames[0]
		bs.GetFileNames = bs.GetFileNames[1:]
	}

	if len(bs.GetErrs) > 0 {
		err = bs.GetErrs[0]
		bs.GetErrs = bs.GetErrs[1:]
	}

	return fileName, err
}

func (bs *FakeBlobstore) CleanUp(fileName string) error {
	bs.CleanUpFileName = fileName
	return bs.CleanUpErr
}

func (bs *FakeBlobstore) Delete(blobId string) error {
	bs.DeleteBlobID = blobId
	return bs.DeleteErr
}

func (bs *FakeBlobstore) Create(fileName string) (string, error) {
	bs.CreateFileNames = append(bs.CreateFileNames, fileName)

	if bs.CreateCallBack != nil {
		bs.CreateCallBack()
	}

	blobID, err := bs.CreateBlobID, bs.CreateErr

	if len(bs.CreateBlobIDs) > 0 {
		blobID = bs.CreateBlobIDs[0]
		bs.CreateBlobIDs = bs.CreateBlobIDs[1:]
	}

	if len(bs.CreateErrs) > 0 {
		err = bs.CreateErrs[0]
		bs.CreateErrs = bs.CreateErrs[1:]
	}

	return blobID, err
}

func (bs *FakeBlobstore) Validate() error {
	return bs.ValidateError
}
