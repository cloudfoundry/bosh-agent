package crypto

import "io"

type DigestProvider interface {
	CreateFromStream(reader io.Reader, algorithm DigestAlgorithm) (Digest, error)
}

type Digest interface {
	VerifyingDigest

	Algorithm() DigestAlgorithm
	Digest() string
	String() string
	Compare(Digest) int // comparing two digests against one another to see which is stronger (e.g SHA256 vs. SHA1)
}

type VerifyingDigest interface {
	Verify(Digest) error
}

type MultipleDigest interface {
	Digests() []Digest
}
