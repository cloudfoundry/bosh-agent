package crypto

import (
	"strings"
	"sort"
	"io"
	"errors"
	"fmt"
)

type MultipleDigest struct {
	digests []Digest
}

func MustNewMultipleDigest(digests ...Digest) MultipleDigest {
	if len(digests) == 0 {
		panic("no digests have been provided")
	}
	return MultipleDigest{digests}
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

func (m MultipleDigest) String() string {
	return m.strongestDigest().String()
}

func (m MultipleDigest) Algorithm() Algorithm {
	if len(m.digests) == 0 {
		return DigestAlgorithmSHA1
	}

	return m.strongestDigest().Algorithm()
}

func (m MultipleDigest) Verify(reader io.Reader) error {
	err := m.validate()
	if err != nil {
		return err
	}
	return m.strongestDigest().Verify(reader)
}

func (m MultipleDigest) validate() error {
	if len(m.digests) == 0 {
		return errors.New("no digests have been provided")
	}

	sort.Sort(ByStrongest{m.digests})
	var previousDigest Digest
	previousDigest = digestImpl{}
	for _, value := range m.digests {
		if value.Algorithm().Compare(previousDigest.Algorithm()) == 0 && value.String() != previousDigest.String() {
			return errors.New(fmt.Sprintf("multiple digests of the same algorithm with different checksums. Algorthim: '%s', digests: '%v'", value.Algorithm(), m.digests))
		}

		previousDigest = value
	}
	return nil
}

func (m MultipleDigest) strongestDigest() (Digest) {
	if len(m.digests) == 0 {
		panic("no digests have been provided")
	}
	sort.Sort(ByStrongest{m.digests})

	return m.digests[len(m.digests) - 1]
}

func (m *MultipleDigest) UnmarshalJSON(data []byte) error {
	digestString := string(data)
	digestString = strings.Replace(digestString, `"`, "", -1)
	multiDigest, err := ParseMultipleDigestString(digestString)

	if err != nil {
		return err
	}

	err = multiDigest.validate()
	if err != nil {
		return err
	}

	*m = multiDigest

	return nil
}

