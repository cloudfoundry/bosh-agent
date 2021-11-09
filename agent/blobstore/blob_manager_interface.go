package blobstore

import (
	"io"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . BlobManagerInterface

type BlobManagerInterface interface {
	Fetch(blobID string) (boshsys.File, int, error)
	Write(blobID string, reader io.Reader) error
	GetPath(blobID string, digest boshcrypto.Digest) (string, error)
	Delete(blobID string) error
	BlobExists(blobID string) bool
}
