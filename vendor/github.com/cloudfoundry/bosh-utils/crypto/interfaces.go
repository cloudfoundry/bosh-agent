package crypto

import "io"

type DigestProvider interface {
	CreateFromStream(reader io.Reader, algorithm DigestAlgorithm) (Digest, error)
}

type Digest interface {
	Verify(Digest) error
	Algorithm() Algorithm
	String() string
}

type Algorithm interface {
	Compare(Algorithm) int
	CreateDigest(io.Reader) (Digest, error)
}
