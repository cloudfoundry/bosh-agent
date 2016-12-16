package crypto

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"strings"
)

var (
	DigestAlgorithmSHA1 Algorithm = algorithmSHA1{Name: "sha1"}
	DigestAlgorithmSHA256 Algorithm = algorithmSHA256{Name: "sha256"}
	DigestAlgorithmSHA512 Algorithm = algorithmSHA512{Name: "sha512"}
)

func CreateHashFromAlgorithm(algorithm DigestAlgorithm) (hash.Hash, error) {
	switch algorithm {
	case DigestAlgorithm("sha1"):
		return sha1.New(), nil
	case DigestAlgorithm("sha256"):
		return sha256.New(), nil
	case DigestAlgorithm("sha512"):
		return sha512.New(), nil
	}

	return nil, errors.New(fmt.Sprintf("Unrecognized digest algorithm: %s", algorithm))
}

func ParseDigestString(digest string) (Digest, error) {
	if len(digest) == 0 {
		return nil, errors.New("Can not parse empty string.")
	}

	pieces := strings.SplitN(digest, ":", 2)

	if len(pieces) == 1 {
		// historically digests were only sha1 and did not include a prefix.
		// continue to support that behavior.
		pieces = []string{"sha1", pieces[0]}
	}

	switch pieces[0] {
	case string("sha1"), string("sha256"), string("sha512"):
		digestAlgo, _ := NewAlgorithm(pieces[0])
		return NewDigest(digestAlgo, pieces[1]), nil
	default:
		return nil, errors.New(fmt.Sprintf("Unrecognized digest algorithm: %s. Supported algorithms: sha1, sha256, sha512", pieces[0]))
	}
}

func ParseMultipleDigestString(multipleDigest string) (MultipleDigest, error) {
	pieces := strings.Split(multipleDigest, ";")

	digests := []Digest{}

	for _, digest := range pieces {
		parsedDigest, err := ParseDigestString(digest)
		if err == nil {
			digests = append(digests, parsedDigest)
		}
	}

	if len(digests) == 0 {
		return MultipleDigest{}, errors.New("No recognizable digest algorithm found. Supported algorithms: sha1, sha256, sha512")
	}

	return MultipleDigest{digests: digests}, nil
}
