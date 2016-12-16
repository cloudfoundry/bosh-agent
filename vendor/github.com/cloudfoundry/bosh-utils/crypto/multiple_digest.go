package crypto

import (
	"errors"
	"fmt"
	"strings"
	"sort"
)

type MultipleDigestImpl struct {
	digests []Digest
}

type Digests []Digest
type ByStrongest struct{ Digests }

func (s Digests) Len() int {
	return len(s)
}
func (s Digests) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByStrongest) Less(i, j int) bool {
	if (s.Digests[i].Algorithm().Compare(s.Digests[j].Algorithm())) < 0 {
		return true
	}
	return false
}

func (m MultipleDigestImpl) String() string {
	sort.Sort(ByStrongest{m.digests})
	return m.digests[len(m.digests) - 1].String()
}

func (m MultipleDigestImpl) Algorithm() Algorithm {
	if len(m.digests) == 0 {
		return DigestAlgorithmSHA1
	}

	sort.Sort(ByStrongest{m.digests})

	return m.digests[len(m.digests) - 1].Algorithm()
}

func (m MultipleDigestImpl) Verify(digest Digest) error {
	for _, candidateDigest := range m.digests {
		if candidateDigest.Verify(digest) == nil {
			return nil
		}
	}

	return errors.New(fmt.Sprintf("No digest found that matches %s", digest.String()))
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

