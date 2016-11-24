package crypto

type DigestProvider interface {
	CreateFromFile(path string, algorithm DigestAlgorithm) (Digest, error)
}

type Digest interface {
	Algorithm() DigestAlgorithm
	Digest() string
	String() string
	Verify(Digest) error
}
