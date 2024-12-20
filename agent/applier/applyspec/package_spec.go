package applyspec

import (
	models "github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
	"github.com/cloudfoundry/bosh-utils/crypto"
)

type PackageSpec struct {
	Name             string                `json:"name"`
	Version          string                `json:"version"`
	Sha1             crypto.MultipleDigest `json:"sha1"`
	BlobstoreID      string                `json:"blobstore_id"`
	SignedURL        string                `json:"signed_url"`
	BlobstoreHeaders map[string]string     `json:"blobstore_headers"`
}

func (s *PackageSpec) AsPackage() models.Package {
	return models.Package{
		Name:    s.Name,
		Version: s.Version,
		Source: models.Source{
			Sha1:             s.Sha1,
			SignedURL:        s.SignedURL,
			BlobstoreID:      s.BlobstoreID,
			BlobstoreHeaders: s.BlobstoreHeaders,
		},
	}
}
