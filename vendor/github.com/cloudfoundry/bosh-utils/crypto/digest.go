package crypto

import (
	"errors"
	"fmt"
	"hash"
	"strings"

	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
)

type DigestAlgorithm string

const (
	DigestAlgorithmSHA1   DigestAlgorithm = "sha1"
	DigestAlgorithmSHA256 DigestAlgorithm = "sha256"
	DigestAlgorithmSHA512 DigestAlgorithm = "sha512"
)

type digestImpl struct {
	algorithm DigestAlgorithm
	digest    string
}

func (c digestImpl) Algorithm() DigestAlgorithm {
	return c.algorithm
}

func (c digestImpl) Digest() string {
	return c.digest
}

func (c digestImpl) String() string {
	if c.algorithm == DigestAlgorithmSHA1 {
		return c.digest
	}

	return fmt.Sprintf("%s:%s", c.algorithm, c.digest)
}

func (c digestImpl) Verify(Digest Digest) error {
	if c.algorithm != Digest.Algorithm() {
		return errors.New(fmt.Sprintf(`Expected %s algorithm but received %s`, c.algorithm, Digest.Algorithm()))
	} else if c.digest != Digest.Digest() {
		return errors.New(fmt.Sprintf(`Expected %s digest "%s" but received "%s"`, c.algorithm, c.digest, Digest.Digest()))
	}

	return nil
}

func NewDigest(algorithm DigestAlgorithm, digest string) Digest {
	return digestImpl{
		algorithm: algorithm,
		digest:    digest,
	}
}

func CreateHashFromAlgorithm(algorithm DigestAlgorithm) (hash.Hash, error) {
	switch algorithm {
	case DigestAlgorithmSHA1:
		return sha1.New(), nil
	case DigestAlgorithmSHA256:
		return sha256.New(), nil
	case DigestAlgorithmSHA512:
		return sha512.New(), nil
	}

	return nil, errors.New(fmt.Sprintf("Unrecognized digest algorithm: %s", algorithm))
}

func ParseDigestString(digest string) (Digest, error) {
	pieces := strings.SplitN(digest, ":", 2)

	if len(pieces) == 1 {
		// historically digests were only sha1 and did not include a prefix.
		// continue to support that behavior.
		pieces = []string{"sha1", pieces[0]}
	}

	switch pieces[0] {
	case "sha1", "sha256", "sha512":
		return NewDigest(DigestAlgorithm(pieces[0]), pieces[1]), nil
	default:
		return nil, errors.New(fmt.Sprintf("Unrecognized digest algorithm: %s", pieces[0]))
	}

	return nil, errors.New(fmt.Sprintf("Parsing digest: %s", digest))
}
