package models

import "github.com/cloudfoundry/bosh-utils/crypto"

type Source struct {
	Sha1          crypto.MultipleDigest
	BlobstoreID   string
	PathInArchive string
}
