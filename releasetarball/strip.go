package releasetarball

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cloudfoundry/bosh-cli/v7/release/manifest"
)

const jobSpecFilename = "job.MF"

// Mirrors the field bosh-cli's release/job/manifest.Manifest reads from job.MF;
// depending on that package would pull yaml.v2 into the agent.
type jobSpec struct {
	Packages []string `yaml:"packages"`
}

// The per-job "packages" field in release.MF is not a substitute: real releases
// ship it empty while each job.MF declares its packages correctly.
func jobPackageNames(releaseTarballPath string) (map[string]bool, error) {
	names := map[string]bool{}

	err := walkTarballFiles(releaseTarballPath, func(name string, _ *tar.Header, r io.Reader) (bool, error) {
		if path.Base(path.Dir(name)) != "jobs" || !strings.HasSuffix(name, ".tgz") {
			return true, nil
		}

		spec, err := readJobSpec(r)
		if err != nil {
			return false, fmt.Errorf("failed to read %s from %s: %w", jobSpecFilename, name, err)
		}

		for _, pkgName := range spec.Packages {
			names[pkgName] = true
		}

		return true, nil
	})

	return names, err
}

func readJobSpec(r io.Reader) (jobSpec, error) {
	var spec jobSpec

	gr, err := gzip.NewReader(r)
	if err != nil {
		return spec, err
	}

	tr := tar.NewReader(gr)

	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return spec, fmt.Errorf("%s not found", jobSpecFilename)
			}
			return spec, err
		}

		if path.Base(h.Name) != jobSpecFilename {
			continue
		}

		buf, err := io.ReadAll(tr)
		if err != nil {
			return spec, err
		}

		return spec, yaml.Unmarshal(buf, &spec)
	}
}

// A package reachable only as a build dependency must still be compiled, but
// only packages a job declares are ever installed on an instance.
func selectPackages(m manifest.Manifest, jobPackages map[string]bool, strip bool) (compile, ship map[string]bool, err error) {
	if len(m.Packages) == 0 {
		return map[string]bool{}, map[string]bool{}, nil
	}

	if !strip || len(jobPackages) == 0 {
		all := make(map[string]bool, len(m.Packages))
		for _, p := range m.Packages {
			all[p.Name] = true
		}
		return all, all, nil
	}

	dependencies := make(map[string][]string, len(m.Packages))
	for _, p := range m.Packages {
		dependencies[p.Name] = p.Dependencies
	}

	compile = make(map[string]bool, len(m.Packages))

	var visit func(string, string) error

	visit = func(name, requiredBy string) error {
		if compile[name] {
			return nil
		}

		deps, found := dependencies[name]
		if !found {
			return fmt.Errorf("package %q required by %s is not in the release manifest", name, requiredBy)
		}

		compile[name] = true

		for _, dep := range deps {
			if err := visit(dep, fmt.Sprintf("package %q", name)); err != nil {
				return err
			}
		}

		return nil
	}

	for name := range jobPackages {
		if err := visit(name, "a job spec"); err != nil {
			return nil, nil, err
		}
	}

	return compile, jobPackages, nil
}

// A dependency with no corresponding package entry is rejected by both bosh-cli
// and the director, so excluded names must be dropped here too.
func shippedDependencies(deps []string, ship map[string]bool) []string {
	filtered := make([]string, 0, len(deps))
	for _, dep := range deps {
		if ship[dep] {
			filtered = append(filtered, dep)
		}
	}
	return filtered
}
