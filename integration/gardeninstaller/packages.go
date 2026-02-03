package gardeninstaller

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// JobManifest represents the job.MF from a BOSH job.
type JobManifest struct {
	Name       string            `yaml:"name"`
	Packages   []string          `yaml:"packages"`
	Templates  map[string]string `yaml:"templates"`
	Properties map[string]struct {
		Description string      `yaml:"description"`
		Default     interface{} `yaml:"default"`
	} `yaml:"properties"`
}

// extractPackages extracts compiled packages from the release tarball
// and streams them to the target via the driver.
// It reads the garden job manifest to determine which packages are needed,
// then extracts those packages to /var/vcap/packages/<name> on the target.
func (i *Installer) extractPackages() ([]string, error) {
	// First, read the garden job manifest to get package list
	jobManifest, err := i.readJobManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to read job manifest: %w", err)
	}

	i.log("Garden job requires packages: %v", jobManifest.Packages)

	// Extract each package and stream to target
	packagesDir := filepath.Join(i.cfg.BaseDir, "packages")
	for _, pkgName := range jobManifest.Packages {
		pkgPath := filepath.Join(packagesDir, pkgName)

		// Create package directory on target
		if err := i.driver.MkdirAll(pkgPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create package dir %s: %w", pkgPath, err)
		}

		// Extract and stream package to target
		if err := i.extractAndStreamPackage(pkgName, pkgPath); err != nil {
			return nil, fmt.Errorf("failed to extract package %s: %w", pkgName, err)
		}
		i.log("Extracted package: %s", pkgName)
	}

	return jobManifest.Packages, nil
}

// readJobManifest reads the garden job.MF from the release tarball.
func (i *Installer) readJobManifest() (*JobManifest, error) {
	var manifest *JobManifest

	err := walkTarball(i.cfg.ReleaseTarballPath, func(name string, _ *tar.Header, r io.Reader) (bool, error) {
		// Look for jobs/garden.tgz
		if name != "jobs/garden.tgz" && name != "./jobs/garden.tgz" {
			return true, nil
		}

		// Read the inner tarball into memory
		innerData, err := io.ReadAll(r)
		if err != nil {
			return false, fmt.Errorf("failed to read garden.tgz: %w", err)
		}

		// Extract job.MF from the inner tarball
		manifest, err = extractJobManifestFromTarball(innerData)
		if err != nil {
			return false, fmt.Errorf("failed to extract job manifest: %w", err)
		}

		return false, nil // Stop walking
	})

	if err != nil {
		return nil, err
	}
	if manifest == nil {
		return nil, fmt.Errorf("garden job not found in release tarball")
	}

	return manifest, nil
}

// extractJobManifestFromTarball extracts job.MF from a job tarball.
func extractJobManifestFromTarball(data []byte) (*JobManifest, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		name := strings.TrimPrefix(header.Name, "./")
		if name == "job.MF" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			var manifest JobManifest
			if err := yaml.Unmarshal(data, &manifest); err != nil {
				return nil, err
			}
			return &manifest, nil
		}
	}

	return nil, fmt.Errorf("job.MF not found in job tarball")
}

// extractAndStreamPackage extracts a package from the release tarball
// and streams it to the target via the driver.
func (i *Installer) extractAndStreamPackage(pkgName, destDir string) error {
	pkgTgzName := fmt.Sprintf("compiled_packages/%s.tgz", pkgName)
	pkgTgzNameDotSlash := "./" + pkgTgzName

	var found bool
	err := walkTarball(i.cfg.ReleaseTarballPath, func(name string, _ *tar.Header, r io.Reader) (bool, error) {
		if name != pkgTgzName && name != pkgTgzNameDotSlash {
			return true, nil
		}

		found = true

		// Read the package tarball into memory
		pkgData, err := io.ReadAll(r)
		if err != nil {
			return false, fmt.Errorf("failed to read package tarball: %w", err)
		}

		// Stream the package tarball to the target
		if err := i.driver.StreamTarball(bytes.NewReader(pkgData), destDir); err != nil {
			return false, fmt.Errorf("failed to stream package to target: %w", err)
		}

		return false, nil // Stop walking
	})

	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("package %s not found in release tarball", pkgName)
	}

	return nil
}

// extractJobTemplates extracts non-ERB template files from the garden job tarball
// and streams them to the target. ERB files need to be rendered separately.
func (i *Installer) extractJobTemplates() error {
	// Files to extract (non-ERB templates that can be used as-is)
	filesToExtract := map[string]string{
		"templates/bin/overlay-xfs-setup": filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "overlay-xfs-setup"),
		"templates/bin/pre-start":         filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "pre-start"),
		"templates/bin/auplink":           filepath.Join(i.cfg.BaseDir, "jobs", "garden", "bin", "auplink"),
	}

	err := walkTarball(i.cfg.ReleaseTarballPath, func(name string, _ *tar.Header, r io.Reader) (bool, error) {
		// Look for jobs/garden.tgz
		if name != "jobs/garden.tgz" && name != "./jobs/garden.tgz" {
			return true, nil
		}

		// Read the inner tarball into memory
		innerData, err := io.ReadAll(r)
		if err != nil {
			return false, fmt.Errorf("failed to read garden.tgz: %w", err)
		}

		// Extract files from the inner tarball
		if err := i.extractFilesFromJobTarball(innerData, filesToExtract); err != nil {
			return false, err
		}

		return false, nil // Stop walking
	})

	return err
}

// extractFilesFromJobTarball extracts specific files from a job tarball.
func (i *Installer) extractFilesFromJobTarball(data []byte, files map[string]string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		name := strings.TrimPrefix(header.Name, "./")

		// Check if this file should be extracted
		destPath, ok := files[name]
		if !ok {
			continue
		}

		// Read file content
		content, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", name, err)
		}

		// Scripts in bin/ should be executable. The templates in the job tarball
		// don't have the executable bit set since BOSH would normally process them.
		mode := int64(0755)
		if !strings.Contains(name, "/bin/") {
			mode = int64(header.Mode)
			if mode == 0 {
				mode = 0644
			}
		}

		// Write to target
		if err := i.driver.WriteFile(destPath, content, mode); err != nil {
			return fmt.Errorf("failed to write %s: %w", destPath, err)
		}
		i.log("Extracted template: %s -> %s", name, destPath)
	}

	return nil
}

// walkTarball walks through a gzipped tarball, calling fn for each file.
// If fn returns false, walking stops. Similar to filepath.WalkDir but for tarballs.
func walkTarball(tarballPath string, fn func(name string, header *tar.Header, r io.Reader) (bool, error)) error {
	f, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Try gzip first, fall back to uncompressed
	var tr *tar.Reader
	gr, err := gzip.NewReader(bufio.NewReader(f))
	if err != nil {
		// Not gzipped, try uncompressed
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		tr = tar.NewReader(f)
	} else {
		defer gr.Close()
		tr = tar.NewReader(gr)
	}

	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		// Normalize path - remove leading ./
		name := path.Clean(header.Name)
		if strings.HasPrefix(name, "./") {
			name = name[2:]
		}

		keepGoing, err := fn(name, header, tr)
		if err != nil {
			return err
		}
		if !keepGoing {
			return nil
		}
	}
}
