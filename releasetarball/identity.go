package releasetarball

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
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

func dependencyKey(name string, all []manifest.PackageRef) (string, error) {
	packages := make(map[string]manifest.PackageRef, len(all))
	for _, p := range all {
		packages[p.Name] = p
	}
	root, ok := packages[name]
	if !ok {
		return "", fmt.Errorf("unknown package %q", name)
	}

	var arrayify func(manifest.PackageRef) ([]any, error)
	arrayify = func(p manifest.PackageRef) ([]any, error) {
		result := []any{p.Name, p.Version}
		if len(p.Dependencies) > 0 {
			dependencies := slices.Clone(p.Dependencies)
			slices.Sort(dependencies)
			children := make([]any, 0, len(dependencies))
			for _, dependency := range dependencies {
				child, ok := packages[dependency]
				if !ok {
					return nil, fmt.Errorf("unknown dependency %q of package %q", dependency, p.Name)
				}
				childKey, err := arrayify(child)
				if err != nil {
					return nil, err
				}
				children = append(children, childKey)
			}
			result = append(result, children)
		}
		return result, nil
	}

	dependencies := slices.Clone(root.Dependencies)
	slices.Sort(dependencies)
	key := make([]any, 0, len(dependencies))
	for _, dependency := range dependencies {
		p, ok := packages[dependency]
		if !ok {
			return "", fmt.Errorf("unknown dependency %q of package %q", dependency, name)
		}
		dependencyKey, err := arrayify(p)
		if err != nil {
			return "", err
		}
		key = append(key, dependencyKey)
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(key); err != nil {
		return "", err
	}
	return strings.TrimSuffix(output.String(), "\n"), nil
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
		key, err := dependencyKey(p.Name, all)
		if err != nil {
			return nil, err
		}

		rewritten := false
		var checkClosure func(manifest.PackageRef) error
		checkClosure = func(current manifest.PackageRef) error {
			for _, dependency := range current.Dependencies {
				child, ok := packages[dependency]
				if !ok {
					return fmt.Errorf("unknown dependency %q of package %q", dependency, current.Name)
				}
				if !shipped[dependency] {
					rewritten = true
				}
				if err := checkClosure(child); err != nil {
					return err
				}
			}
			return nil
		}
		if err := checkClosure(p); err != nil {
			return nil, err
		}

		identity := rewrittenPkg{
			Fingerprint:         p.Fingerprint,
			OriginalFingerprint: p.Fingerprint,
			Dependencies:        p.Dependencies,
			Rewritten:           rewritten,
		}
		if rewritten {
			var payload strings.Builder
			payload.WriteString(p.Fingerprint)
			payload.WriteByte('\n')
			payload.WriteString(key)
			payload.WriteByte('\n')
			keptDependencies := make([]string, 0, len(p.Dependencies))
			for _, dependency := range p.Dependencies {
				if shipped[dependency] {
					keptDependencies = append(keptDependencies, dependency)
				}
			}
			slices.Sort(keptDependencies)
			payload.WriteString(strings.Join(keptDependencies, ","))

			switch len(p.Fingerprint) {
			case 40:
				digest := sha1.Sum([]byte(payload.String()))
				identity.Fingerprint = hex.EncodeToString(digest[:])
			case 64:
				digest := sha256.Sum256([]byte(payload.String()))
				identity.Fingerprint = hex.EncodeToString(digest[:])
			default:
				return nil, fmt.Errorf("package %q has unsupported fingerprint length %d", p.Name, len(p.Fingerprint))
			}
			identity.Dependencies = keptDependencies
		}
		result[p.Name] = identity
	}
	return result, nil
}
