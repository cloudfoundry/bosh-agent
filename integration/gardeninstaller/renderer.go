package gardeninstaller

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// erbRendererRb is the Ruby script that evaluates ERB templates.
// This is embedded from bosh-cli's templatescompiler/erbrenderer/erb_renderer.rb
//
//go:embed erb_renderer.rb
var erbRendererRb string

// Renderer renders ERB templates from a BOSH job using Ruby.
// It wraps the bosh-cli's ERB rendering approach but is simplified for our use case.
type Renderer struct {
	// BaseDir is the BOSH installation directory (e.g., /var/vcap)
	BaseDir string

	// Properties holds the job properties for template rendering
	Properties *Properties

	// Debug enables verbose logging
	Debug bool
}

// NewRenderer creates a new ERB template renderer.
func NewRenderer(baseDir string, props *Properties, debug bool) *Renderer {
	return &Renderer{
		BaseDir:    baseDir,
		Properties: props,
		Debug:      debug,
	}
}

// TemplateEvaluationContext is the context passed to the Ruby ERB renderer.
// This structure matches what bosh-cli's erb_renderer.rb expects.
type TemplateEvaluationContext struct {
	// Index is the instance index (always 0 for our use case)
	Index int `json:"index"`

	// ID is the instance ID
	ID string `json:"id"`

	// AZ is the availability zone
	AZ string `json:"az"`

	// Bootstrap indicates if this is the bootstrap instance
	Bootstrap bool `json:"bootstrap"`

	// Job contains job metadata
	Job JobContext `json:"job"`

	// Deployment is the deployment name
	Deployment string `json:"deployment"`

	// Address is the instance address
	Address string `json:"address,omitempty"`

	// Networks contains network information
	Networks map[string]NetworkContext `json:"networks"`

	// GlobalProperties are top-level manifest properties (not used)
	GlobalProperties map[string]interface{} `json:"global_properties"`

	// ClusterProperties are instance group properties (not used)
	ClusterProperties map[string]interface{} `json:"cluster_properties"`

	// JobProperties are the job-specific properties
	JobProperties map[string]interface{} `json:"job_properties"`

	// DefaultProperties are the defaults from the job spec
	DefaultProperties map[string]interface{} `json:"default_properties"`
}

// JobContext contains job metadata.
type JobContext struct {
	Name string `json:"name"`
}

// NetworkContext contains network information.
type NetworkContext struct {
	IP      string `json:"ip"`
	Netmask string `json:"netmask"`
	Gateway string `json:"gateway"`
}

// BuildContext creates a TemplateEvaluationContext for rendering templates.
// It combines the properties with the job spec defaults.
func (r *Renderer) BuildContext(jobDefaults map[string]interface{}) (*TemplateEvaluationContext, error) {
	propsMap, err := r.Properties.ToMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert properties to map: %w", err)
	}

	ctx := &TemplateEvaluationContext{
		Index:     0,
		ID:        "gardeninstaller-test",
		AZ:        "z1",
		Bootstrap: true,
		Job: JobContext{
			Name: "garden",
		},
		Deployment: "gardeninstaller",
		Networks: map[string]NetworkContext{
			"default": {IP: "127.0.0.1"},
		},
		GlobalProperties:  map[string]interface{}{},
		ClusterProperties: propsMap,
		JobProperties:     nil, // When nil, ERB uses cluster_properties merged with defaults
		DefaultProperties: jobDefaults,
	}

	return ctx, nil
}

// RenderTemplate renders a single ERB template file.
// srcPath is the path to the .erb template file.
// dstPath is the path where the rendered output should be written.
// context is the template evaluation context.
func (r *Renderer) RenderTemplate(srcPath, dstPath string, context *TemplateEvaluationContext) error {
	// Create a temporary directory for the rendering
	tmpDir, err := os.MkdirTemp("", "erb-renderer-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write the Ruby renderer script
	rendererScriptPath := filepath.Join(tmpDir, "erb_renderer.rb")
	if err := os.WriteFile(rendererScriptPath, []byte(erbRendererRb), 0644); err != nil {
		return fmt.Errorf("failed to write renderer script: %w", err)
	}

	// Write the context as JSON
	contextPath := filepath.Join(tmpDir, "context.json")
	contextBytes, err := json.Marshal(context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}
	if err := os.WriteFile(contextPath, contextBytes, 0644); err != nil {
		return fmt.Errorf("failed to write context: %w", err)
	}

	if r.Debug {
		fmt.Printf("[renderer] Rendering %s -> %s\n", srcPath, dstPath)
		fmt.Printf("[renderer] Context: %s\n", string(contextBytes))
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination dir: %w", err)
	}

	// Run Ruby to render the template
	cmd := exec.Command("ruby", rendererScriptPath, contextPath, srcPath, dstPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ruby ERB rendering failed: %w\nOutput: %s", err, string(output))
	}

	if r.Debug {
		fmt.Printf("[renderer] Successfully rendered %s\n", dstPath)
	}

	return nil
}

// RenderTemplates renders multiple ERB templates from a job.
// templateDir is the directory containing the extracted job templates.
// outputDir is the directory where rendered files should be written.
// templates is a map of source template paths (relative to templateDir) to destination paths (relative to outputDir).
func (r *Renderer) RenderTemplates(templateDir, outputDir string, templates map[string]string, jobDefaults map[string]interface{}) error {
	context, err := r.BuildContext(jobDefaults)
	if err != nil {
		return fmt.Errorf("failed to build context: %w", err)
	}

	for srcRel, dstRel := range templates {
		srcPath := filepath.Join(templateDir, srcRel)
		dstPath := filepath.Join(outputDir, dstRel)

		if err := r.RenderTemplate(srcPath, dstPath, context); err != nil {
			return fmt.Errorf("failed to render %s: %w", srcRel, err)
		}
	}

	return nil
}
