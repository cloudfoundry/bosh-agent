package crypto

import (
	"errors"
	"fmt"
	"io"

)

type DigestAlgorithm string

type digestImpl struct {
	algorithm Algorithm
	digest    string
}


func (c digestImpl) Algorithm() Algorithm {
	return c.algorithm
}

func (c digestImpl) String() string {
	if c.algorithm == DigestAlgorithmSHA1 {
		return c.digest
	}

	return fmt.Sprintf("%s:%s", c.algorithm, c.digest)
}

func (c digestImpl) Verify(reader io.Reader) error {
	otherDigest, err := c.Algorithm().CreateDigest(reader)
	if err != nil {
		return err
	}

	if otherDigest, ok := otherDigest.(digestImpl); ok {
		if c.digest != otherDigest.digest {
			return errors.New(fmt.Sprintf(`Expected %s digest "%s" but received "%s"`, c.algorithm, c.digest, otherDigest.String()))
		}
	} else {
		return errors.New(fmt.Sprintf(`Unknown digest to verify against %v`, otherDigest.String()))
	}

	return nil
}

func NewDigest(algorithm Algorithm, digest string) digestImpl {
	return digestImpl{
		algorithm: algorithm,
		digest:    digest,
	}
}
