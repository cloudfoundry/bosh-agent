package crypto

import (
	"io"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

func NewAlgorithm(algorithm string) (Algorithm, error) {
	switch algorithm {
	case "sha1":
		return DigestAlgorithmSHA1, nil
	case "sha256":
		return DigestAlgorithmSHA256, nil
	case "sha512":
		return DigestAlgorithmSHA512, nil
	}
	return nil, bosherr.Errorf("Unknown algorithim '%s'", algorithm)
}

type algorithmSHA512 struct {
	Name string
}

func (a algorithmSHA512)  String() string {
	return a.Name
}

func (a algorithmSHA512)  Compare(other Algorithm) int {
	if _, ok := other.(algorithmSHA512); ok {
		return 0
	}
	return 1
}

func (a algorithmSHA512)  CreateDigest(reader io.Reader) (Digest, error) {
	return digestProviderImpl{}.CreateFromStream(reader, DigestAlgorithm(a.Name))
}

type algorithmSHA256 struct {
	Name string
}

func (a algorithmSHA256)  CreateDigest(reader io.Reader) (Digest, error) {
	return digestProviderImpl{}.CreateFromStream(reader, DigestAlgorithm(a.Name))

}

func (a algorithmSHA256)  String() string {
	return a.Name
}

func (a algorithmSHA256)  Compare(other Algorithm) int {
	if _, ok := other.(algorithmSHA1); ok {
		return 1
	} else if _, ok = other.(algorithmSHA512); ok {
		return -1
	} else if _, ok = other.(algorithmSHA256); ok {
		return 0
	}
	return 1
}

type algorithmSHA1 struct {
	Name string
}

func (a algorithmSHA1)  CreateDigest(reader io.Reader) (Digest, error) {
	return digestProviderImpl{}.CreateFromStream(reader, DigestAlgorithm(a.Name))

}

func (a algorithmSHA1)  String() string {
	return a.Name
}

func (a algorithmSHA1)  Compare(other Algorithm) int {
	if _, ok := other.(algorithmSHA1); ok {
		return 0
	} else if _, ok = other.(algorithmSHA512); ok {
		return -1
	} else if _, ok = other.(algorithmSHA256); ok {
		return -1
	}
	return 1
}
