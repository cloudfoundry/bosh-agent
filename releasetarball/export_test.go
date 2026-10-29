package releasetarball

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	boshmodels "github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
	"github.com/cloudfoundry/bosh-cli/v7/release/manifest"
)

// These adapters expose private helpers only to the external test package.
// They are compiled by go test and are not part of the public API.
type IdentityPkg = manifest.PackageRef

type RewrittenIdentity = rewrittenPkg

func DependencyKeyForTest(name string, all []IdentityPkg) (string, error) {
	packages := make(map[string]manifest.PackageRef, len(all))
	for _, p := range all {
		packages[p.Name] = p
	}
	root, ok := packages[name]
	if !ok {
		return "", fmt.Errorf("unknown package %q", name)
	}
	tree, _, err := dependencyTree(root, packages, nil)
	if err != nil {
		return "", err
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(tree); err != nil {
		return "", err
	}
	return strings.TrimSuffix(output.String(), "\n"), nil
}

func RewriteIdentitiesForTest(all []IdentityPkg, shipped map[string]bool) (map[string]RewrittenIdentity, error) {
	return rewriteIdentities(all, shipped)
}

func ShippedPackagesForTest(path string, m manifest.Manifest) (map[string]bool, error) {
	return shippedPackages(path, m)
}

func WriteCompiledReleaseForTest(m manifest.Manifest, outputDirectory, stemcellSlug, blobsDirectory, initialTarball string, sourcePackages []manifest.PackageRef, compiledPackages []boshmodels.Package, shipped map[string]bool) (string, error) {
	m.Packages = sourcePackages
	return writeCompiledRelease(m, outputDirectory, stemcellSlug, blobsDirectory, initialTarball, compiledPackages, CompileOptions{ExcludeCompileTimePackages: true}, shipped)
}
