package models

import (
	"github.com/cloudfoundry/bosh-utils/crypto"
	"log"
)

type Package struct {
	Name    string
	Version string
	Source  Source
}

func (s Package) BundleName() string {
	return s.Name
}

func (s Package) BundleVersion() string {
	var digest string

	if s.Source.Sha1 != nil {
		preferredDigest, err := crypto.PreferredDigest(s.Source.Sha1)
		if err != nil {
			log.Panicf("Err that can't be returned!! %v", err)
		}
		digest = preferredDigest.Digest()
	}

	return s.Version + "-" + digest
}
