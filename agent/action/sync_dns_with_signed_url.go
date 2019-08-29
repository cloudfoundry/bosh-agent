package action

import (
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

type SyncDNSWithSignedURLRequest struct {
	SignedURL   string                    `json:"signed_url"`
	MultiDigest boshcrypto.MultipleDigest `json:"multi_digest"`
	Version     uint64                    `json:"version"`
}

type SyncDNSWithSignedURL struct{}

func (a SyncDNSWithSignedURL) Run(request SyncDNSWithSignedURLRequest) (string, error) {
	return "", nil
}
