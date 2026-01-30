//go:build linux

package firewall

import (
	"fmt"
	"net"
	gonetURL "net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

// NftablesFirewallCLI implements Manager and NatsFirewallHook using the nft CLI.
// This is used as a fallback when the netlink-based implementation fails in
// nested container environments where netlink can return EOVERFLOW errors.
type NftablesFirewallCLI struct {
	cgroupResolver CgroupResolver
	cgroupVersion  CgroupVersion
	logger         boshlog.Logger
	logTag         string

	// State stored during SetupAgentRules for use in BeforeConnect
	enableNATSFirewall bool
	agentCgroup        ProcessCgroup
}

// NewNftablesFirewallCLI creates a new CLI-based nftables firewall manager
func NewNftablesFirewallCLI(logger boshlog.Logger) (Manager, error) {
	return NewNftablesFirewallCLIWithDeps(&realCgroupResolver{}, logger)
}

// NewNftablesFirewallCLIWithDeps creates a CLI-based firewall manager with injected dependencies
func NewNftablesFirewallCLIWithDeps(cgroupResolver CgroupResolver, logger boshlog.Logger) (Manager, error) {
	f := &NftablesFirewallCLI{
		cgroupResolver: cgroupResolver,
		logger:         logger,
		logTag:         "NftablesFirewallCLI",
	}

	// Detect cgroup version at construction time
	var err error
	f.cgroupVersion, err = cgroupResolver.DetectVersion()
	if err != nil {
		return nil, bosherr.WrapError(err, "Detecting cgroup version")
	}

	f.logger.Info(f.logTag, "Initialized with cgroup version %d (using CLI)", f.cgroupVersion)

	return f, nil
}

// runNft executes an nft command and returns any error
func (f *NftablesFirewallCLI) runNft(args ...string) error {
	f.logger.Debug(f.logTag, "Running: nft %s", strings.Join(args, " "))
	cmd := exec.Command("nft", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return bosherr.WrapErrorf(err, "nft %s: %s", strings.Join(args, " "), string(output))
	}
	return nil
}

// SetupAgentRules sets up the agent's own firewall exceptions during bootstrap.
func (f *NftablesFirewallCLI) SetupAgentRules(mbusURL string, enableNATSFirewall bool) error {
	f.logger.Info(f.logTag, "Setting up agent firewall rules (enableNATSFirewall=%v)", enableNATSFirewall)

	// Store for later use in BeforeConnect
	f.enableNATSFirewall = enableNATSFirewall

	// Get agent's own cgroup path/classid (cache for later use)
	agentCgroup, err := f.cgroupResolver.GetProcessCgroup(os.Getpid(), f.cgroupVersion)
	if err != nil {
		return bosherr.WrapError(err, "Getting agent cgroup")
	}
	f.agentCgroup = agentCgroup

	f.logger.Debug(f.logTag, "Agent cgroup: version=%d path=%s classid=%d",
		agentCgroup.Version, agentCgroup.Path, agentCgroup.ClassID)

	// Create table (delete first to ensure clean state)
	_ = f.runNft("delete", "table", "inet", TableName) // ignore error if table doesn't exist
	if err := f.runNft("add", "table", "inet", TableName); err != nil {
		return bosherr.WrapError(err, "Creating nftables table")
	}

	// Create monit chain with priority -1 (runs before base firewall at priority 0)
	if err := f.runNft("add", "chain", "inet", TableName, MonitChainName,
		"{ type filter hook output priority -1; policy accept; }"); err != nil {
		return bosherr.WrapError(err, "Creating monit chain")
	}

	// Add monit access rule for agent
	if err := f.addMonitRuleCLI(agentCgroup); err != nil {
		return bosherr.WrapError(err, "Adding agent monit rule")
	}

	// Create NATS chain if enabled
	if enableNATSFirewall {
		if err := f.runNft("add", "chain", "inet", TableName, NATSChainName,
			"{ type filter hook output priority -1; policy accept; }"); err != nil {
			return bosherr.WrapError(err, "Creating NATS chain")
		}
	}

	f.logger.Info(f.logTag, "Successfully set up monit firewall rules")
	return nil
}

// BeforeConnect implements NatsFirewallHook. It resolves the NATS URL and updates
// firewall rules before each connection/reconnection attempt.
func (f *NftablesFirewallCLI) BeforeConnect(mbusURL string) error {
	if !f.enableNATSFirewall {
		return nil
	}

	// Parse URL to get host and port
	host, port, err := parseNATSURL(mbusURL)
	if err != nil {
		// Not an error for https URLs or empty URLs
		f.logger.Debug(f.logTag, "Skipping NATS firewall: %s", err)
		return nil
	}

	// Resolve host to IP addresses (or use directly if already an IP)
	var addrs []net.IP
	if ip := net.ParseIP(host); ip != nil {
		// Already an IP address, no DNS needed
		addrs = []net.IP{ip}
	} else {
		// Hostname - try DNS resolution
		addrs, err = net.LookupIP(host)
		if err != nil {
			// DNS failed - log warning but don't fail
			f.logger.Warn(f.logTag, "DNS resolution failed for %s: %s", host, err)
			return nil
		}
	}

	f.logger.Debug(f.logTag, "Updating NATS firewall rules for %s:%d (resolved to %v)", host, port, addrs)

	// Flush NATS chain (removes old rules)
	if err := f.runNft("flush", "chain", "inet", TableName, NATSChainName); err != nil {
		// Chain might not exist yet
		f.logger.Debug(f.logTag, "Could not flush NATS chain: %s", err)
	}

	// Add rules for each resolved IP
	for _, addr := range addrs {
		if err := f.addNATSAllowRuleCLI(addr, port); err != nil {
			return bosherr.WrapError(err, "Adding NATS allow rule")
		}
		if err := f.addNATSBlockRuleCLI(addr, port); err != nil {
			return bosherr.WrapError(err, "Adding NATS block rule")
		}
	}

	f.logger.Info(f.logTag, "Updated NATS firewall rules for %s:%d", host, port)
	return nil
}

// AllowService opens firewall for the calling process to access a service
func (f *NftablesFirewallCLI) AllowService(service Service, callerPID int) error {
	if !IsAllowedService(service) {
		return fmt.Errorf("service %q not in allowed list", service)
	}

	f.logger.Info(f.logTag, "Allowing service %s for PID %d", service, callerPID)

	// Get caller's cgroup
	callerCgroup, err := f.cgroupResolver.GetProcessCgroup(callerPID, f.cgroupVersion)
	if err != nil {
		return bosherr.WrapError(err, "Getting caller cgroup")
	}

	f.logger.Debug(f.logTag, "Caller cgroup: version=%d path=%s classid=%d",
		callerCgroup.Version, callerCgroup.Path, callerCgroup.ClassID)

	switch service {
	case ServiceMonit:
		if err := f.addMonitRuleCLI(callerCgroup); err != nil {
			return bosherr.WrapError(err, "Adding monit rule for caller")
		}
	default:
		return fmt.Errorf("service %q not implemented", service)
	}

	f.logger.Info(f.logTag, "Successfully added firewall exception for %s", service)
	return nil
}

// Cleanup removes all agent-managed firewall rules
func (f *NftablesFirewallCLI) Cleanup() error {
	f.logger.Info(f.logTag, "Cleaning up firewall rules")
	return f.runNft("delete", "table", "inet", TableName)
}

// addMonitRuleCLI adds a rule allowing the specified cgroup to access monit
func (f *NftablesFirewallCLI) addMonitRuleCLI(cgroup ProcessCgroup) error {
	// Build rule: <cgroup match> ip daddr 127.0.0.1 tcp dport 2822 accept
	var rule string
	if f.cgroupVersion == CgroupV2 {
		// Cgroup v2: match on cgroup path
		rule = fmt.Sprintf("socket cgroupv2 level 2 \"%s\" ip daddr 127.0.0.1 tcp dport %d accept",
			cgroup.Path, MonitPort)
	} else {
		// Cgroup v1: match on classid
		classID := cgroup.ClassID
		if classID == 0 {
			classID = MonitClassID
		}
		rule = fmt.Sprintf("meta cgroup %d ip daddr 127.0.0.1 tcp dport %d accept",
			classID, MonitPort)
	}

	return f.runNft("add", "rule", "inet", TableName, MonitChainName, rule)
}

// addNATSAllowRuleCLI adds a rule allowing the agent's cgroup to access NATS
func (f *NftablesFirewallCLI) addNATSAllowRuleCLI(addr net.IP, port int) error {
	var rule string
	ipStr := addr.String()

	if f.cgroupVersion == CgroupV2 {
		rule = fmt.Sprintf("socket cgroupv2 level 2 \"%s\" ip daddr %s tcp dport %d accept",
			f.agentCgroup.Path, ipStr, port)
	} else {
		classID := f.agentCgroup.ClassID
		if classID == 0 {
			classID = NATSClassID
		}
		rule = fmt.Sprintf("meta cgroup %d ip daddr %s tcp dport %d accept",
			classID, ipStr, port)
	}

	return f.runNft("add", "rule", "inet", TableName, NATSChainName, rule)
}

// addNATSBlockRuleCLI adds a rule blocking everyone else from accessing NATS
func (f *NftablesFirewallCLI) addNATSBlockRuleCLI(addr net.IP, port int) error {
	// No cgroup match - applies to all processes
	ipStr := addr.String()
	rule := fmt.Sprintf("ip daddr %s tcp dport %d drop", ipStr, port)
	return f.runNft("add", "rule", "inet", TableName, NATSChainName, rule)
}

// parseNATSURLCLI parses a NATS URL and returns host and port
// This is a copy of parseNATSURL to avoid circular dependencies
func parseNATSURLCLI(mbusURL string) (string, int, error) {
	if mbusURL == "" || strings.HasPrefix(mbusURL, "https://") {
		return "", 0, fmt.Errorf("skipping URL: %s", mbusURL)
	}

	u, err := gonetURL.Parse(mbusURL)
	if err != nil {
		return "", 0, err
	}

	if u.Hostname() == "" {
		return "", 0, fmt.Errorf("empty hostname in URL")
	}

	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Hostname()
		portStr = "4222"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("parsing port: %w", err)
	}

	return host, port, nil
}
