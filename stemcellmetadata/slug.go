//go:build !linux

package stemcellmetadata

import "log"

func SlugParts() (_ string, _ string, _ string, _ error) {
	log.Fatal("func readStemcellSlug (in package stemcellmetadata) is not implemented for GOOS != linux")
	return
}
