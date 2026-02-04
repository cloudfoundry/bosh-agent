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

// ExtractJobTemplatesToLocal extracts all templates from the garden job tarball
// to a local directory for ERB rendering. Returns the path to the extracted templates
// directory and the job manifest.
func ExtractJobTemplatesToLocal(releaseTarballPath string) (templateDir string, manifest *JobManifest, err error) {
	// Create a temporary directory for the templates
	templateDir, err = os.MkdirTemp("", "garden-job-templates-")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Find and extract garden.tgz from the release tarball
	err = walkTarball(releaseTarballPath, func(name string, _ *tar.Header, r io.Reader) (bool, error) {
		// Look for jobs/garden.tgz
		if name != "jobs/garden.tgz" && name != "./jobs/garden.tgz" {
			return true, nil
		}

		// Read the inner tarball into memory
		innerData, err := io.ReadAll(r)
		if err != nil {
			return false, fmt.Errorf("failed to read garden.tgz: %w", err)
		}

		// Extract all files from the inner tarball
		manifest, err = extractAllFromJobTarball(innerData, templateDir)
		if err != nil {
			return false, fmt.Errorf("failed to extract job tarball: %w", err)
		}

		return false, nil // Stop walking
	})

	if err != nil {
		os.RemoveAll(templateDir)
		return "", nil, err
	}
	if manifest == nil {
		os.RemoveAll(templateDir)
		return "", nil, fmt.Errorf("garden job not found in release tarball")
	}

	return templateDir, manifest, nil
}

// extractAllFromJobTarball extracts all files from a job tarball to a directory.
// Returns the job manifest parsed from job.MF.
func extractAllFromJobTarball(data []byte, destDir string) (*JobManifest, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	var manifest *JobManifest
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
		destPath := filepath.Join(destDir, name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create dir %s: %w", destPath, err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return nil, fmt.Errorf("failed to create parent dir for %s: %w", destPath, err)
			}

			// Read file content
			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", name, err)
			}

			// Parse job.MF if this is it
			if name == "job.MF" {
				manifest = &JobManifest{}
				if err := yaml.Unmarshal(content, manifest); err != nil {
					return nil, fmt.Errorf("failed to parse job.MF: %w", err)
				}
			}

			// Write the file
			mode := os.FileMode(header.Mode)
			if mode == 0 {
				mode = 0644
			}
			if err := os.WriteFile(destPath, content, mode); err != nil {
				return nil, fmt.Errorf("failed to write %s: %w", destPath, err)
			}
		}
	}

	if manifest == nil {
		return nil, fmt.Errorf("job.MF not found in job tarball")
	}

	return manifest, nil
}

// GetJobPropertyDefaults extracts the default values from the job manifest properties.
// Returns a flat map with dotted keys (e.g., "garden.listen_network") suitable for
// use as default_properties in the ERB context. The erb_renderer.rb uses copy_property()
// which expects flat dotted keys that it splits to navigate nested structures.
func (m *JobManifest) GetJobPropertyDefaults() map[string]interface{} {
	defaults := make(map[string]interface{})

	for propName, propDef := range m.Properties {
		// Keep the property name as a flat dotted key
		// The erb_renderer.rb copy_property() will split it to navigate nested structures
		defaults[propName] = propDef.Default
	}

	return defaults
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
