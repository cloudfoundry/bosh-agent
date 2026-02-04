package gardeninstaller

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

// defaultPropertiesYAML contains the default property values for rendering garden job templates.
// These are the properties that differ from the garden job spec defaults, tuned for integration tests.
// The structure mirrors what BOSH would provide in a deployment manifest.
//
//go:embed properties_defaults.yml
var defaultPropertiesYAML string

// Properties holds the garden job properties used for ERB template rendering.
// The structure matches the BOSH manifest property format expected by the ERB templates.
type Properties struct {
	Garden  GardenProperties  `yaml:"garden"`
	Grootfs GrootfsProperties `yaml:"grootfs"`
	BPM     BPMProperties     `yaml:"bpm"`
}

// GardenProperties contains garden-specific configuration.
type GardenProperties struct {
	ListenNetwork             string   `yaml:"listen_network"`
	ListenAddress             string   `yaml:"listen_address"`
	AllowHostAccess           bool     `yaml:"allow_host_access"`
	DestroyContainersOnStart  bool     `yaml:"destroy_containers_on_start"`
	LogLevel                  string   `yaml:"log_level"`
	DefaultContainerRootfs    string   `yaml:"default_container_rootfs"`
	NetworkPool               string   `yaml:"network_pool"`
	AppArmorProfile           string   `yaml:"apparmor_profile"`
	ContainerdMode            bool     `yaml:"containerd_mode"`
	DNSServers                []string `yaml:"dns_servers"`
	MaxContainers             int      `yaml:"max_containers"`
	DebugListenAddress        string   `yaml:"debug_listen_address,omitempty"`
	DefaultContainerGraceTime string   `yaml:"default_container_grace_time"`
	RuntimePlugin             string   `yaml:"runtime_plugin"`
	IptablesBinDir            string   `yaml:"iptables_bin_dir"`
	NetworkMTU                int      `yaml:"network_mtu"`
	CleanupProcessDirsOnWait  bool     `yaml:"cleanup_process_dirs_on_wait"`
}

// GrootfsProperties contains grootfs-specific configuration.
type GrootfsProperties struct {
	LogLevel                      string `yaml:"log_level"`
	SkipMount                     bool   `yaml:"skip_mount"`
	ReservedSpaceForOtherJobsInMB int    `yaml:"reserved_space_for_other_jobs_in_mb"`
	RoutineGC                     bool   `yaml:"routine_gc"`
}

// BPMProperties contains BPM-specific configuration.
type BPMProperties struct {
	Enabled bool `yaml:"enabled"`
}

// DefaultProperties returns the default properties for garden job rendering.
// These defaults are suitable for integration tests and nested Garden installations.
func DefaultProperties() (*Properties, error) {
	var props Properties
	if err := yaml.Unmarshal([]byte(defaultPropertiesYAML), &props); err != nil {
		return nil, err
	}
	return &props, nil
}

// PropertiesFromConfig creates Properties from an Installer Config.
// This merges the Config values with the defaults from the YAML.
func PropertiesFromConfig(cfg Config) (*Properties, error) {
	props, err := DefaultProperties()
	if err != nil {
		return nil, err
	}

	// Override with Config values
	if cfg.ListenNetwork != "" {
		props.Garden.ListenNetwork = cfg.ListenNetwork
	}
	if cfg.ListenAddress != "" {
		props.Garden.ListenAddress = cfg.ListenAddress
	}
	if cfg.NetworkPool != "" {
		props.Garden.NetworkPool = cfg.NetworkPool
	}
	props.Garden.AllowHostAccess = cfg.AllowHostAccess
	props.Garden.DestroyContainersOnStart = cfg.DestroyOnStart

	// Override containerd mode if explicitly set
	// This is critical for nested installations where containerd cannot run
	if cfg.ContainerdMode != nil {
		props.Garden.ContainerdMode = *cfg.ContainerdMode
	}

	// Update paths based on BaseDir
	if cfg.BaseDir != "" && cfg.BaseDir != "/var/vcap" {
		props.Garden.DefaultContainerRootfs = cfg.BaseDir + "/packages/busybox/busybox-1.36.1.tar"
		props.Garden.RuntimePlugin = cfg.BaseDir + "/packages/runc/bin/runc"
		props.Garden.IptablesBinDir = cfg.BaseDir + "/packages/iptables/sbin"
	}

	return props, nil
}

// ToMap converts Properties to a map[string]interface{} for use with the ERB renderer.
// The map structure matches what BOSH provides in the template evaluation context.
func (p *Properties) ToMap() (map[string]interface{}, error) {
	// Marshal to YAML then unmarshal to map to get the correct structure
	data, err := yaml.Marshal(p)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
