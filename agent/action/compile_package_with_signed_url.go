package action

import (
	boshcomp "github.com/cloudfoundry/bosh-agent/agent/compiler"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

type CompilePackageWithSignedURLRequest struct {
	PackageGetSignedURL string `json:"package_get_signed_url"`
	UploadSignedURL     string `json:"upload_signed_url"`

	MultiDigest boshcrypto.MultipleDigest `json:"multi_digest"`
	Name        string                    `json:"name"`
	Version     string                    `json:"version"`
	Deps        boshcomp.Dependencies     `json:"deps"`
}

type CompilePackageWithSignedURLResponse struct {
	SHA1Digest string `json:"sha1_digest"`
}

type CompilePackageWithSignedURL struct{}

func (a CompilePackageWithSignedURL) Run(request CompilePackageWithSignedURLRequest) (CompilePackageWithSignedURLResponse, error) {
	return CompilePackageWithSignedURLResponse{}, nil
}
