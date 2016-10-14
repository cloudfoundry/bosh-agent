package blobstore

import (
    "io"
    boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type BlobManagerInterface interface {

    Fetch(blobID string) (boshsys.File, error, int)

    Write(blobID string, reader io.Reader) error
}
