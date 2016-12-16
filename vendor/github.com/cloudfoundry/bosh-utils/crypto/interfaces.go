package crypto

import "io"

type DigestProvider interface {
	CreateFromStream(reader io.Reader, algorithm DigestAlgorithm) (Digest, error)
}

type Digest interface {
	Verify(io.Reader) error
	Algorithm() Algorithm
	String() string
}

type Algorithm interface {
	Compare(Algorithm) int
	CreateDigest(io.Reader) (Digest, error)
	String() string
}
