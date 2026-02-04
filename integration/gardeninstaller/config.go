package gardeninstaller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// erbTemplates maps source template paths (relative to the extracted job templates dir)
// to destination paths (relative to BaseDir). Only .erb files are rendered with Ruby.
var erbTemplates = map[string]string{
	"templates/bin/envs.erb":                             "jobs/garden/bin/envs",
	"templates/bin/garden_start.erb":                     "jobs/garden/bin/garden_start",
	"templates/bin/garden_stop.erb":                      "jobs/garden/bin/garden_stop",
	"templates/bin/grootfs-utils.erb":                    "jobs/garden/bin/grootfs-utils",
	"templates/bin/containerd_utils.erb":                 "jobs/garden/bin/containerd_utils",
	"templates/config/config.ini.erb":                    "jobs/garden/config/config.ini",
	"templates/config/grootfs_config.yml.erb":            "jobs/garden/config/grootfs_config.yml",
	"templates/config/privileged_grootfs_config.yml.erb": "jobs/garden/config/privileged_grootfs_config.yml",
	"templates/config/containerd.toml.erb":               "jobs/garden/config/containerd.toml",
}

// staticTemplates maps source template paths to destination paths for files that
// don't need ERB rendering (they're just copied as-is).
var staticTemplates = map[string]string{
	"templates/bin/overlay-xfs-setup": "jobs/garden/bin/overlay-xfs-setup",
	"templates/bin/pre-start":         "jobs/garden/bin/pre-start",
	"templates/bin/post-start":        "jobs/garden/bin/post-start",
	"templates/bin/auplink":           "jobs/garden/bin/auplink",
	"templates/bin/garden_ctl":        "jobs/garden/bin/garden_ctl",
	"templates/config/garden-default": "jobs/garden/config/garden-default",
	"templates/config/garden.service": "jobs/garden/config/garden.service",
}

// generateConfigs renders ERB templates and copies static templates from the garden job.
// It extracts templates from the release tarball, renders ERB templates locally using Ruby,
// and streams the rendered files to the target.
func (i *Installer) generateConfigs() error {
	// Extract job templates to a local temp directory
	templateDir, manifest, err := ExtractJobTemplatesToLocal(i.cfg.ReleaseTarballPath)
	if err != nil {
		return fmt.Errorf("failed to extract job templates: %w", err)
	}
	defer os.RemoveAll(templateDir)

	i.log("Extracted job templates to %s", templateDir)

	// Get job property defaults from the manifest
	jobDefaults := manifest.GetJobPropertyDefaults()

	// Create properties from config
	props, err := PropertiesFromConfig(i.cfg)
	if err != nil {
		return fmt.Errorf("failed to create properties: %w", err)
	}

	// Create the renderer
	renderer := NewRenderer(i.cfg.BaseDir, props, i.cfg.Debug)

	// Create a temp directory for rendered output
	outputDir, err := os.MkdirTemp("", "garden-rendered-")
	if err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	defer os.RemoveAll(outputDir)

	// Render ERB templates
	i.log("Rendering ERB templates...")
	if err := renderer.RenderTemplates(templateDir, outputDir, erbTemplates, jobDefaults); err != nil {
		return fmt.Errorf("failed to render ERB templates: %w", err)
	}

	// Copy static templates to output directory
	i.log("Copying static templates...")
	for src, dst := range staticTemplates {
		srcPath := filepath.Join(templateDir, src)
		dstPath := filepath.Join(outputDir, dst)

		// Read source file
		content, err := os.ReadFile(srcPath)
		if err != nil {
			// Some static templates may not exist in all versions, skip them
			if os.IsNotExist(err) {
				i.log("Skipping missing static template: %s", src)
				continue
			}
			return fmt.Errorf("failed to read static template %s: %w", src, err)
		}

		// Post-process overlay-xfs-setup to add --store-size-bytes flag
		// This is needed because in containers, disk space is limited and grootfs
		// will fail to create XFS backing stores without a minimum size specified.
		if dst == "jobs/garden/bin/overlay-xfs-setup" {
			content = i.patchOverlayXfsSetup(content)
		}

		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return fmt.Errorf("failed to create dir for %s: %w", dstPath, err)
		}

		// Write to output directory
		if err := os.WriteFile(dstPath, content, 0755); err != nil {
			return fmt.Errorf("failed to write static template %s: %w", dst, err)
		}
	}

	// Stream all rendered/copied files to the target
	i.log("Streaming rendered configs to target...")
	if err := i.streamRenderedConfigs(outputDir); err != nil {
		return fmt.Errorf("failed to stream configs to target: %w", err)
	}

	return nil
}

// streamRenderedConfigs walks the output directory and streams all files to the target.
func (i *Installer) streamRenderedConfigs(outputDir string) error {
	return filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(outputDir, path)
		if err != nil {
			return err
		}

		// Destination path on target
		destPath := filepath.Join(i.cfg.BaseDir, relPath)

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Determine file mode - scripts in bin/ should be executable
		mode := int64(0644)
		if strings.Contains(relPath, "/bin/") {
			mode = 0755
		}

		// Ensure parent directory exists on target
		if err := i.driver.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create dir for %s: %w", destPath, err)
		}

		// Write to target
		if err := i.driver.WriteFile(destPath, content, mode); err != nil {
			return fmt.Errorf("failed to write %s: %w", destPath, err)
		}

		i.log("Streamed config: %s", destPath)
		return nil
	})
}

// patchOverlayXfsSetup modifies the overlay-xfs-setup script to add --store-size-bytes
// to grootfs init-store commands. This is necessary because in containers, grootfs
// calculates very small backing store sizes based on available disk space, which
// causes mkfs.xfs to fail ("agsize too small, need at least 4096 blocks").
//
// The store size is configured via Config.StoreSizeBytes.
func (i *Installer) patchOverlayXfsSetup(content []byte) []byte {
	if i.cfg.StoreSizeBytes <= 0 {
		// No patching needed if store size is not configured
		return content
	}

	script := string(content)

	// Add --store-size-bytes to the unprivileged store init
	// Original: grootfs --config ${config_path} init-store \
	// Patched:  grootfs --config ${config_path} init-store --store-size-bytes <size> \
	script = strings.Replace(script,
		`grootfs --config ${config_path} init-store \`,
		fmt.Sprintf(`grootfs --config ${config_path} init-store --store-size-bytes %d \`, i.cfg.StoreSizeBytes),
		1) // Only replace the first occurrence (unprivileged store)

	// Add --store-size-bytes to the privileged store init
	// Original: grootfs --config ${config_path} init-store
	// Patched:  grootfs --config ${config_path} init-store --store-size-bytes <size>
	// Note: This one doesn't have a trailing backslash
	script = strings.Replace(script,
		"grootfs --config ${config_path} init-store\n",
		fmt.Sprintf("grootfs --config ${config_path} init-store --store-size-bytes %d\n", i.cfg.StoreSizeBytes),
		1)

	i.log("Patched overlay-xfs-setup with --store-size-bytes %d", i.cfg.StoreSizeBytes)
	return []byte(script)
}
