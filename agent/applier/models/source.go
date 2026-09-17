package models

import (
	"fmt"

	"github.com/cloudfoundry/bosh-utils/crypto"
	"github.com/cloudfoundry/bosh-utils/redact"
)

type Source struct {
	Sha1             crypto.Digest
	BlobstoreID      string
	PathInArchive    string
	SignedURL        redact.Secret
	BlobstoreHeaders map[string]string
}

// String keeps Source loggable without leaking credentials. SignedURL is a
// redact.Secret so it self-redacts (here and anywhere else it is printed);
// BlobstoreHeaders values are masked here because they can carry auth headers.
func (s Source) String() string {
	headers := s.BlobstoreHeaders
	if len(headers) > 0 {
		redacted := make(map[string]string, len(headers))
		for k := range headers {
			redacted[k] = redact.Placeholder
		}
		headers = redacted
	}

	return fmt.Sprintf(
		"{Sha1:%s BlobstoreID:%s PathInArchive:%s SignedURL:%v BlobstoreHeaders:%v}",
		s.Sha1, s.BlobstoreID, s.PathInArchive, s.SignedURL, headers,
	)
}
