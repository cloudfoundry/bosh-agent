package releasetarball

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	"github.com/cloudfoundry/bosh-cli/v7/release/job/manifest"
	releasemanifest "github.com/cloudfoundry/bosh-cli/v7/release/manifest"
	"gopkg.in/yaml.v2"
)

func shippedPackages(releaseTarballPath string, m releasemanifest.Manifest) (map[string]bool, error) {
	jobs := make(map[string]bool, len(m.Jobs))
	for _, job := range m.Jobs {
		jobs[job.Name] = false
	}
	knownPackages := make(map[string]bool, len(m.Packages)+len(m.CompiledPkgs))
	for _, p := range m.Packages {
		knownPackages[p.Name] = true
	}
	for _, p := range m.CompiledPkgs {
		knownPackages[p.Name] = true
	}
	shipped := make(map[string]bool)
	if err := walkTarballFiles(releaseTarballPath, func(name string, _ *tar.Header, r io.Reader) (bool, error) {
		name = strings.TrimPrefix(name, "./")
		if !strings.HasPrefix(name, "jobs/") || !strings.HasSuffix(name, ".tgz") {
			return true, nil
		}
		jobName := strings.TrimSuffix(strings.TrimPrefix(name, "jobs/"), ".tgz")
		jobManifest, err := readJobManifest(r)
		if err != nil {
			return false, fmt.Errorf("failed to read job %q in release %q: %w", jobName, releaseTarballPath, err)
		}
		if _, declared := jobs[jobName]; declared {
			jobs[jobName] = true
		}
		for _, packageName := range jobManifest.Packages {
			if !knownPackages[packageName] {
				return false, fmt.Errorf("job %q in release %q references unknown package %q", jobName, releaseTarballPath, packageName)
			}
			shipped[packageName] = true
		}
		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to read release %q: %w", releaseTarballPath, err)
	}

	for _, job := range m.Jobs {
		if !jobs[job.Name] {
			return nil, fmt.Errorf("release %q is missing job archive for job %q", releaseTarballPath, job.Name)
		}
	}
	if len(m.Packages) >= 1 && len(shipped) == 0 {
		return nil, fmt.Errorf("release %q has packages but no shipped packages in job.MF files", releaseTarballPath)
	}
	return shipped, nil
}

func readJobManifest(r io.Reader) (manifest.Manifest, error) {
	buffered := bufio.NewReader(r)
	header, err := buffered.Peek(2)
	if err != nil && err != io.EOF {
		return manifest.Manifest{}, err
	}
	var archive io.Reader = buffered
	if len(header) == 2 && header[0] == 0x1f && header[1] == 0x8b {
		gr, err := gzip.NewReader(buffered)
		if err != nil {
			return manifest.Manifest{}, err
		}
		defer closeAndIgnoreErr(gr)
		archive = gr
	}
	tr := tar.NewReader(archive)
	var manifestContents []byte
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return manifest.Manifest{}, err
		}
		if strings.TrimPrefix(h.Name, "./") != "job.MF" {
			continue
		}
		if manifestContents != nil {
			return manifest.Manifest{}, fmt.Errorf("job archive has duplicate job.MF entries")
		}
		if h.Typeflag != tar.TypeReg {
			return manifest.Manifest{}, fmt.Errorf("job archive job.MF is not a regular file")
		}
		manifestContents, err = io.ReadAll(tr)
		if err != nil {
			return manifest.Manifest{}, err
		}
	}
	if manifestContents == nil {
		return manifest.Manifest{}, fmt.Errorf("job archive has no job.MF")
	}
	var jobManifest manifest.Manifest
	if err := yaml.Unmarshal(manifestContents, &jobManifest); err != nil {
		return manifest.Manifest{}, err
	}
	return jobManifest, nil
}
