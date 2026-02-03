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
	// mbusURL is passed for configuration but NATS rules are set up later via BeforeConnect hook.
	// enableNATSFirewall controls whether NATS rules will be created (Jammy: true, Noble: false).
	SetupAgentRules(mbusURL string, enableNATSFirewall bool) error

	// AllowService opens firewall for the calling process's cgroup to access a service.
	// Returns error if service is not in AllowedServices.
	// Called by BOSH-deployed jobs via "bosh-agent firewall-allow <service>" when they
	// need to interact with local services directly (e.g., monit API for controlled failover).
	// On Jammy, the legacy permit_monit_access helper wraps this for backward compatibility.
	AllowService(service Service, callerPID int) error

	// Cleanup removes all agent-managed firewall rules.
	// Called during agent shutdown (optional).
	Cleanup() error
}

// NatsFirewallHook is called before each NATS connection/reconnection attempt.
// Implementations should resolve DNS and update firewall rules atomically.
// This interface is implemented by Manager implementations that support NATS firewall.
//
//counterfeiter:generate . NatsFirewallHook
type NatsFirewallHook interface {
	// BeforeConnect resolves the NATS URL and updates firewall rules.
	// Called before initial connect and before each reconnection attempt.
	// This allows DNS to be re-resolved on reconnect, supporting HA failover
	// where the director may have moved to a different IP.
	// Returns nil on success or if NATS firewall is disabled.
	// Errors are logged but should not prevent connection attempts.
	BeforeConnect(mbusURL string) error
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
