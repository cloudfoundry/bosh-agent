package releasetarball

import (
	boshmodels "github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
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

func ShippedPackagesForTest(path string, m manifest.Manifest) (map[string]bool, error) {
	return shippedPackages(path, m)
}

func WriteCompiledReleaseForTest(m manifest.Manifest, outputDirectory, stemcellSlug, blobsDirectory, initialTarball string, sourcePackages []manifest.PackageRef, compiledPackages []boshmodels.Package, shipped map[string]bool) (string, error) {
	return writeCompiledRelease(m, outputDirectory, stemcellSlug, blobsDirectory, initialTarball, sourcePackages, compiledPackages, CompileOptions{ExcludeCompileTimePackages: true}, shipped)
}
