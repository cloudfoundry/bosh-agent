package crypto

import (
	"errors"
	"fmt"
)

type DigestAlgorithm string

type digestImpl struct {
	algorithm 		DigestAlgorithm
	digest    		string
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

func (c digestImpl) Compare(digest Digest) int {
	switch c.algorithm {
	case DigestAlgorithmSHA1:
		if digest.Algorithm() == DigestAlgorithmSHA1 {
			return 0
		} else {
			return -1
		}
	case DigestAlgorithmSHA256:
		if digest.Algorithm() == DigestAlgorithmSHA1 {
			return 1
		} else if digest.Algorithm() == DigestAlgorithmSHA256 {
			return 0
		} else {
			return -1
		}
	case DigestAlgorithmSHA512:
		if digest.Algorithm() == DigestAlgorithmSHA512 {
			return 0
		} else {
			return 1
		}
	}
	return 0
}

func NewDigest(algorithm DigestAlgorithm, digest string) digestImpl {
	return digestImpl{
		algorithm: algorithm,
		digest:    digest,
	}
}
