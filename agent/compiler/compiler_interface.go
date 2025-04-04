package compiler

import (
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"

	boshmodels "github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
)

type Compiler interface {
	Compile(pkg Package, deps []boshmodels.Package) (blobID string, digest boshcrypto.Digest, err error)
}

type Package struct {
	BlobstoreID         string `json:"blobstore_id"`
	Name                string
	PackageGetSignedURL string            `json:"package_get_signed_url"`
	UploadSignedURL     string            `json:"upload_signed_url"`
	BlobstoreHeaders    map[string]string `json:"blobstore_headers"`
	Sha1                boshcrypto.MultipleDigest
	Version             string
}

type Dependencies map[string]Package
