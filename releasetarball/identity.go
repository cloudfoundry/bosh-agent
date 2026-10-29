package releasetarball

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cloudfoundry/bosh-cli/v7/release/manifest"
)

type rewrittenPkg struct {
	Fingerprint         string
	OriginalFingerprint string
	Dependencies        []string
	Rewritten           bool
}

// dependencyTree follows the director's KeyGenerator#arrayify, including sorted
// children and omitted empty child arrays. Keep the director vectors in sync:
// https://github.com/cloudfoundry/bosh/blob/0410768ff620384100a277ff0683274970bc2c56/src/bosh-director/lib/bosh/director/key_generator.rb
// Compile checks for cycles before traversing this graph.
func dependencyTree(p manifest.PackageRef, packages map[string]manifest.PackageRef, shipped map[string]bool) ([]any, bool, error) {
	dependencies := slices.Clone(p.Dependencies)
	slices.Sort(dependencies)
	tree := make([]any, 0, len(dependencies))
	trimmed := false
	for _, name := range dependencies {
		child, ok := packages[name]
		if !ok {
			return nil, false, fmt.Errorf("unknown dependency %q of package %q", name, p.Name)
		}
		children, childTrimmed, err := dependencyTree(child, packages, shipped)
		if err != nil {
			return nil, false, err
		}
		entry := []any{child.Name, child.Version}
		if len(children) > 0 {
			entry = append(entry, children)
		}
		tree = append(tree, entry)
		trimmed = trimmed || !shipped[name] || childTrimmed
	}
	return tree, trimmed, nil
}

func rewriteIdentities(all []manifest.PackageRef, shipped map[string]bool) (map[string]rewrittenPkg, error) {
	packages := make(map[string]manifest.PackageRef, len(all))
	for _, p := range all {
		packages[p.Name] = p
	}
	result := make(map[string]rewrittenPkg)
	for _, p := range all {
		if !shipped[p.Name] {
			continue
		}
		tree, trimmed, err := dependencyTree(p, packages, shipped)
		if err != nil {
			return nil, err
		}
		identity := rewrittenPkg{
			Fingerprint:         p.Fingerprint,
			OriginalFingerprint: p.Fingerprint,
			Dependencies:        p.Dependencies,
			Rewritten:           trimmed,
		}
		if trimmed {
			identity.Dependencies = make([]string, 0, len(p.Dependencies))
			for _, name := range p.Dependencies {
				if shipped[name] {
					identity.Dependencies = append(identity.Dependencies, name)
				}
			}
			slices.Sort(identity.Dependencies)
			var key bytes.Buffer
			encoder := json.NewEncoder(&key)
			encoder.SetEscapeHTML(false)
			if err := encoder.Encode(tree); err != nil {
				return nil, err
			}
			// Preserve the existing trimmed identity format: source fingerprint,
			// director dependency JSON, then comma-separated kept dependencies.
			dependencyKey := strings.TrimSuffix(key.String(), "\n")
			keptDependencies := strings.Join(identity.Dependencies, ",")
			payload := p.Fingerprint + "\n" + dependencyKey + "\n" + keptDependencies
			switch len(p.Fingerprint) {
			case 40:
				digest := sha1.Sum([]byte(payload))
				identity.Fingerprint = fmt.Sprintf("%x", digest)
			case 64:
				digest := sha256.Sum256([]byte(payload))
				identity.Fingerprint = fmt.Sprintf("%x", digest)
			default:
				return nil, fmt.Errorf("package %q has unsupported fingerprint length %d", p.Name, len(p.Fingerprint))
			}
		}
		result[p.Name] = identity
	}
	return result, nil
}
