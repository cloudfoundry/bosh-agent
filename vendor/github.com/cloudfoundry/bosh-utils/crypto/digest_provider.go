package crypto

import (
	"fmt"
	"io"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type digestProviderImpl struct {}

func NewDigestProvider() DigestProvider {
	return digestProviderImpl{}
}

func (f digestProviderImpl) CreateFromStream(reader io.Reader, algorithm DigestAlgorithm) (Digest, error) {
	hash, err := CreateHashFromAlgorithm(algorithm)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(hash, reader)
	if err != nil {
		return nil, bosherr.WrapError(err, "Copying file for digest calculation")
	}

	digestAlgorithm, err := NewAlgorithm(string(algorithm))
	if err != nil {
		return nil, err
	}

	return NewDigest(digestAlgorithm, fmt.Sprintf("%x", hash.Sum(nil))), nil
}
