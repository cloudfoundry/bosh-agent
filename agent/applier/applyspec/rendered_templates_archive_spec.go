package applyspec

import (
	"github.com/cloudfoundry/bosh-agent/agent/applier/models"
	"github.com/cloudfoundry/bosh-utils/crypto"
)

type RenderedTemplatesArchiveSpec struct {
	Sha1        crypto.MultipleDigest `json:"sha1"`
	BlobstoreID string                `json:"blobstore_id"`
}

func (s RenderedTemplatesArchiveSpec) AsSource(job models.Job) models.Source {
	return models.Source{
		Sha1:          s.Sha1,
		BlobstoreID:   s.BlobstoreID,
		PathInArchive: job.Name,
	}
}
