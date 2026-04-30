// Package firewall provides nftables-based firewall management for the BOSH agent.
//
// The firewall protects access to:
// - Monit (port 2822 on localhost): Used by the agent to manage job processes
// - NATS (director's message bus): Used for agent-director communication
//
// Security Model:
// The firewall uses UID-based matching (meta skuid 0) to allow only root processes
// to access these services. This blocks non-root BOSH job workloads (vcap user)
// while allowing the agent and operators to access monit/NATS.
//
// This approach is simpler and more reliable than cgroup-based matching, which
// fails in nested container environments due to cgroup filesystem bind-mount issues.
package firewall

import "fmt"

const (
	TableName            = "bosh_agent"
	MonitChainName       = "monit_access"
	MonitJobsChainName   = "monit_access_jobs"
	NATSChainName        = "nats_access"
	MonitPort            = 2822
	MonitAccessLogPrefix = "bosh-monit-access: "
)

var (
	ErrMonitJobsChainNotFound = fmt.Errorf("%s chain not found", MonitJobsChainName)
	ErrBoshTableNotFound      = fmt.Errorf("%s table not found", TableName)
)

// Manager handles firewall setup
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . Manager
type Manager interface {
	// SetupMonitFirewall creates firewall rules to protect monit (port 2822).
	// Only root (UID 0) is allowed to connect.
	SetupMonitFirewall() error

	// EnableMonitAccess enables monit access by adding firewall rules.
	// It first tries to use cgroup-based matching, then falls back to UID-based matching.
	EnableMonitAccess() error

	// SetupNATSFirewall creates firewall rules to protect NATS.
	// Only root (UID 0) is allowed to connect to the resolved NATS address.
	// This method resolves DNS and should be called before each connection attempt.
	SetupNATSFirewall(mbusURL string) error

	// Cleanup closes the nftables connection.
	Cleanup() error
}

// NatsFirewallHook is called by the NATS handler before connection/reconnection.
// This allows DNS to be re-resolved, supporting HA failover scenarios.
//
//counterfeiter:generate . NatsFirewallHook
type NatsFirewallHook interface {
	// BeforeConnect is called before each NATS connection/reconnection attempt.
	// It resolves the NATS URL and updates firewall rules with the resolved IP.
	BeforeConnect(mbusURL string) error
}
