package crypto

import (
	"errors"
	"fmt"
	"strings"
)

type MultipleDigestImpl struct {
	digests []Digest
}

func Verify(m MultipleDigest, digest Digest) error {
	for _, candidateDigest := range m.Digests() {
		if candidateDigest.Algorithm() == digest.Algorithm() {
			return candidateDigest.Verify(digest)
		}
	}

	return errors.New(fmt.Sprintf("No digest found that matches %s", digest.Algorithm()))
}

func (m MultipleDigestImpl) Digests() []Digest {
	return m.digests
}

func NewMultipleDigest(digests ...Digest) MultipleDigestImpl {
	return MultipleDigestImpl{digests: digests}
}

func (m *MultipleDigestImpl) UnmarshalJSON(data []byte) error {
	digestString := string(data)
	digestString = strings.Replace(digestString, `"`, "", -1)
	multiDigest, err := ParseMultipleDigestString(digestString)

	if err != nil {
		return err
	}

	*m = multiDigest
	return nil
}
