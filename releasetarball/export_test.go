package releasetarball

import (
	"github.com/cloudfoundry/bosh-cli/v7/release/manifest"
)

type IdentityPkg = manifest.PackageRef

type RewrittenIdentity = rewrittenPkg

func DependencyKeyForTest(name string, all []IdentityPkg) (string, error) {
	return dependencyKey(name, all)
}

func RewriteIdentitiesForTest(all []IdentityPkg, shipped map[string]bool) (map[string]RewrittenIdentity, error) {
	return rewriteIdentities(all, shipped)
}
