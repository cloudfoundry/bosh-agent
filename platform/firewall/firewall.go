package firewall

// Service represents a local service that can be protected by firewall
type Service string

const (
	ServiceMonit Service = "monit"
	// Future services can be added here
)

// AllowedServices is the list of services that can be requested via CLI
var AllowedServices = []Service{ServiceMonit}

// CgroupVersion represents the cgroup hierarchy version
type CgroupVersion int

const (
	CgroupV1 CgroupVersion = 1
	CgroupV2 CgroupVersion = 2
)

// ProcessCgroup represents a process's cgroup identity
type ProcessCgroup struct {
	Version CgroupVersion
	Path    string // For cgroup v2: full path like "/system.slice/bosh-agent.service"
	ClassID uint32 // For cgroup v1: net_cls classid
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . Manager

// Manager manages firewall rules for local service access
type Manager interface {
	// SetupAgentRules sets up the agent's own firewall exceptions during bootstrap.
	// Called once during agent bootstrap after networking is configured.
	// mbusURL is the NATS URL for setting up NATS firewall rules (Jammy only).
	SetupAgentRules(mbusURL string) error

	// AllowService opens firewall for the calling process to access a service.
	// Returns error if service is not in AllowedServices.
	// Called by external processes via "bosh-agent firewall-allow <service>".
	AllowService(service Service, callerPID int) error

	// Cleanup removes all agent-managed firewall rules.
	// Called during agent shutdown (optional).
	Cleanup() error
}

// IsAllowedService checks if a service is in the allowed list
func IsAllowedService(s Service) bool {
	for _, allowed := range AllowedServices {
		if s == allowed {
			return true
		}
	}
	return false
}
