//go:build linux

package firewall

import (
	"encoding/binary"
	"fmt"
	"net"
	gonetURL "net/url"
	"os"
	"strconv"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

const (
	// BOSH classid namespace: 0xb054XXXX (b054 = "BOSH" leet-ified)
	// 0xb0540001 = monit access (used by stemcell scripts)
	// 0xb0540002 = NATS access (used by agent)
	MonitClassID uint32 = 0xb0540001 // 2958295041
	NATSClassID  uint32 = 0xb0540002 // 2958295042

	TableName      = "bosh_agent"
	MonitChainName = "monit_access"
	NATSChainName  = "nats_access"

	MonitPort = 2822

	// AllowMark is the packet mark used to signal to the base bosh_firewall table
	// that this packet has been allowed by the agent. The base firewall checks for
	// this mark and skips the DROP rule when it's set. This enables cross-table
	// coordination since nftables evaluates each table's chains independently.
	// Mark value: 0xb054 ("BOSH" leet-ified)
	AllowMark uint32 = 0xb054
)

// NftablesConn abstracts the nftables connection for testing
//
//counterfeiter:generate -header ./linux_header.txt . NftablesConn
type NftablesConn interface {
	AddTable(t *nftables.Table) *nftables.Table
	AddChain(c *nftables.Chain) *nftables.Chain
	AddRule(r *nftables.Rule) *nftables.Rule
	DelTable(t *nftables.Table)
	FlushChain(c *nftables.Chain)
	Flush() error
}

// CgroupResolver abstracts cgroup detection for testing
//
//counterfeiter:generate -header ./linux_header.txt . CgroupResolver
type CgroupResolver interface {
	DetectVersion() (CgroupVersion, error)
	GetProcessCgroup(pid int, version CgroupVersion) (ProcessCgroup, error)
	// GetCgroupID returns the cgroup inode ID for the given cgroup path.
	// This is used for nftables "socket cgroupv2" matching, which compares
	// against the cgroup inode ID (not the path string).
	GetCgroupID(cgroupPath string) (uint64, error)
}

// realNftablesConn wraps the actual nftables.Conn
type realNftablesConn struct {
	conn *nftables.Conn
}

func (r *realNftablesConn) AddTable(t *nftables.Table) *nftables.Table {
	return r.conn.AddTable(t)
}

func (r *realNftablesConn) AddChain(c *nftables.Chain) *nftables.Chain {
	return r.conn.AddChain(c)
}

func (r *realNftablesConn) AddRule(rule *nftables.Rule) *nftables.Rule {
	return r.conn.AddRule(rule)
}

func (r *realNftablesConn) DelTable(t *nftables.Table) {
	r.conn.DelTable(t)
}

func (r *realNftablesConn) FlushChain(c *nftables.Chain) {
	r.conn.FlushChain(c)
}

func (r *realNftablesConn) Flush() error {
	return r.conn.Flush()
}

// realCgroupResolver implements CgroupResolver using actual system calls
type realCgroupResolver struct{}

func (r *realCgroupResolver) DetectVersion() (CgroupVersion, error) {
	return DetectCgroupVersion()
}

func (r *realCgroupResolver) GetProcessCgroup(pid int, version CgroupVersion) (ProcessCgroup, error) {
	return GetProcessCgroup(pid, version)
}

func (r *realCgroupResolver) GetCgroupID(cgroupPath string) (uint64, error) {
	return GetCgroupID(cgroupPath)
}

// NftablesFirewall implements Manager and NatsFirewallHook using nftables via netlink
type NftablesFirewall struct {
	conn           NftablesConn
	cgroupResolver CgroupResolver
	cgroupVersion  CgroupVersion
	logger         boshlog.Logger
	logTag         string
	table          *nftables.Table
	monitChain     *nftables.Chain
	natsChain      *nftables.Chain

	// State stored during SetupAgentRules for use in BeforeConnect
	enableNATSFirewall bool
	agentCgroup        ProcessCgroup
}

// NewNftablesFirewall creates a new nftables-based firewall manager
func NewNftablesFirewall(logger boshlog.Logger) (Manager, error) {
	conn, err := nftables.New()
	if err != nil {
		return nil, bosherr.WrapError(err, "Creating nftables connection")
	}

	return NewNftablesFirewallWithDeps(
		&realNftablesConn{conn: conn},
		&realCgroupResolver{},
		logger,
	)
}

// NewNftablesFirewallWithDeps creates a firewall manager with injected dependencies (for testing)
func NewNftablesFirewallWithDeps(conn NftablesConn, cgroupResolver CgroupResolver, logger boshlog.Logger) (Manager, error) {
	f := &NftablesFirewall{
		conn:           conn,
		cgroupResolver: cgroupResolver,
		logger:         logger,
		logTag:         "NftablesFirewall",
	}

	// Detect cgroup version at construction time
	var err error
	f.cgroupVersion, err = cgroupResolver.DetectVersion()
	if err != nil {
		return nil, bosherr.WrapError(err, "Detecting cgroup version")
	}

	f.logger.Info(f.logTag, "Initialized with cgroup version %d", f.cgroupVersion)

	return f, nil
}

// SetupAgentRules sets up the agent's own firewall exceptions during bootstrap.
// Monit rules are set up immediately. NATS rules are set up later via BeforeConnect hook.
func (f *NftablesFirewall) SetupAgentRules(mbusURL string, enableNATSFirewall bool) error {
	f.logger.Info(f.logTag, "Setting up agent firewall rules (enableNATSFirewall=%v)", enableNATSFirewall)

	// Store for later use in BeforeConnect
	f.enableNATSFirewall = enableNATSFirewall

	// Create or get our table
	if err := f.ensureTable(); err != nil {
		return bosherr.WrapError(err, "Creating nftables table")
	}

	// Get agent's own cgroup path/classid (cache for later use)
	agentCgroup, err := f.cgroupResolver.GetProcessCgroup(os.Getpid(), f.cgroupVersion)
	if err != nil {
		return bosherr.WrapError(err, "Getting agent cgroup")
	}
	f.agentCgroup = agentCgroup

	f.logger.Info(f.logTag, "Agent cgroup: version=%d path='%s' classid=%d",
		agentCgroup.Version, agentCgroup.Path, agentCgroup.ClassID)

	// Create monit chain and add monit rule
	if err := f.ensureMonitChain(); err != nil {
		return bosherr.WrapError(err, "Creating monit chain")
	}

	if err := f.addMonitRule(agentCgroup); err != nil {
		return bosherr.WrapError(err, "Adding agent monit rule")
	}

	// Create NATS chain (empty for now - BeforeConnect will populate it)
	if enableNATSFirewall {
		if err := f.ensureNATSChain(); err != nil {
			return bosherr.WrapError(err, "Creating NATS chain")
		}
	}

	// Commit all rules
	if err := f.conn.Flush(); err != nil {
		return bosherr.WrapError(err, "Flushing nftables rules")
	}

	f.logger.Info(f.logTag, "Successfully set up monit firewall rules")
	return nil
}

// BeforeConnect implements NatsFirewallHook. It resolves the NATS URL and updates
// firewall rules before each connection/reconnection attempt.
func (f *NftablesFirewall) BeforeConnect(mbusURL string) error {
	f.logger.Info(f.logTag, "BeforeConnect called: enableNATSFirewall=%v, mbusURL=%s", f.enableNATSFirewall, mbusURL)
	if !f.enableNATSFirewall {
		return nil
	}

	// Parse URL to get host and port
	host, port, err := parseNATSURL(mbusURL)
	if err != nil {
		// Not an error for https URLs or empty URLs
		f.logger.Info(f.logTag, "Skipping NATS firewall: %s", err)
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

	// Ensure NATS chain exists
	if f.natsChain == nil {
		if err := f.ensureNATSChain(); err != nil {
			return bosherr.WrapError(err, "Creating NATS chain")
		}
	}

	// Flush NATS chain (removes old rules)
	f.conn.FlushChain(f.natsChain)

	// Add rules for each resolved IP:
	// 1. ACCEPT rule for agent's cgroup (allows agent to connect)
	// 2. DROP rule for everyone else (blocks malicious workloads)
	for _, addr := range addrs {
		if err := f.addNATSAllowRule(addr, port); err != nil {
			return bosherr.WrapError(err, "Adding NATS allow rule")
		}
		if err := f.addNATSBlockRule(addr, port); err != nil {
			return bosherr.WrapError(err, "Adding NATS block rule")
		}
	}

	// Commit
	if err := f.conn.Flush(); err != nil {
		return bosherr.WrapError(err, "Flushing nftables rules")
	}

	f.logger.Info(f.logTag, "Updated NATS firewall rules for %s:%d", host, port)
	return nil
}

// AllowService opens firewall for the calling process to access a service
func (f *NftablesFirewall) AllowService(service Service, callerPID int) error {
	// Validate service is in allowlist
	if !IsAllowedService(service) {
		return fmt.Errorf("service %q not in allowed list", service)
	}

	f.logger.Info(f.logTag, "Allowing service %s for PID %d", service, callerPID)

	// Ensure table and chain exist
	if err := f.ensureTable(); err != nil {
		return bosherr.WrapError(err, "Ensuring nftables table")
	}
	if err := f.ensureMonitChain(); err != nil {
		return bosherr.WrapError(err, "Ensuring nftables chain")
	}

	// Get caller's cgroup
	callerCgroup, err := f.cgroupResolver.GetProcessCgroup(callerPID, f.cgroupVersion)
	if err != nil {
		return bosherr.WrapError(err, "Getting caller cgroup")
	}

	f.logger.Debug(f.logTag, "Caller cgroup: version=%d path=%s classid=%d",
		callerCgroup.Version, callerCgroup.Path, callerCgroup.ClassID)

	switch service {
	case ServiceMonit:
		if err := f.addMonitRule(callerCgroup); err != nil {
			return bosherr.WrapError(err, "Adding monit rule for caller")
		}
	default:
		return fmt.Errorf("service %q not implemented", service)
	}

	if err := f.conn.Flush(); err != nil {
		return bosherr.WrapError(err, "Flushing nftables rules")
	}

	f.logger.Info(f.logTag, "Successfully added firewall exception for %s", service)
	return nil
}

// Cleanup removes all agent-managed firewall rules
func (f *NftablesFirewall) Cleanup() error {
	f.logger.Info(f.logTag, "Cleaning up firewall rules")

	// Delete our table (this removes all chains and rules in it)
	if f.table != nil {
		f.conn.DelTable(f.table)
	}

	return f.conn.Flush()
}

func (f *NftablesFirewall) ensureTable() error {
	f.table = &nftables.Table{
		Family: nftables.TableFamilyINet,
		Name:   TableName,
	}
	f.conn.AddTable(f.table)
	return nil
}

func (f *NftablesFirewall) ensureMonitChain() error {
	// Priority -1 ensures our ACCEPT rules run before base DROP rules (priority 0)
	priority := nftables.ChainPriority(*nftables.ChainPriorityFilter - 1)

	f.monitChain = &nftables.Chain{
		Name:     MonitChainName,
		Table:    f.table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookOutput,
		Priority: &priority,
		Policy:   policyPtr(nftables.ChainPolicyAccept),
	}
	f.conn.AddChain(f.monitChain)
	return nil
}

func (f *NftablesFirewall) ensureNATSChain() error {
	// Priority -1 ensures our ACCEPT rules run before base DROP rules (priority 0)
	priority := nftables.ChainPriority(*nftables.ChainPriorityFilter - 1)

	f.natsChain = &nftables.Chain{
		Name:     NATSChainName,
		Table:    f.table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookOutput,
		Priority: &priority,
		Policy:   policyPtr(nftables.ChainPolicyAccept),
	}
	f.conn.AddChain(f.natsChain)
	return nil
}

func (f *NftablesFirewall) addMonitRule(cgroup ProcessCgroup) error {
	// Build rule: <cgroup match> + dst 127.0.0.1 + dport 2822 -> set mark + accept
	// The mark signals to the base bosh_firewall table (in a separate table) that
	// this packet was allowed by the agent and should NOT be dropped.
	// This is necessary because nftables evaluates each table independently -
	// an ACCEPT in one table doesn't prevent other tables from also evaluating.
	exprs, err := f.buildCgroupMatchExprs(cgroup)
	if err != nil {
		return fmt.Errorf("building cgroup match expressions: %w", err)
	}
	exprs = append(exprs, f.buildLoopbackDestExprs()...)
	exprs = append(exprs, f.buildTCPDestPortExprs(MonitPort)...)
	exprs = append(exprs, f.buildSetMarkExprs()...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.monitChain,
		Exprs: exprs,
	})

	return nil
}

func (f *NftablesFirewall) addNATSAllowRule(addr net.IP, port int) error {
	// Build rule: <cgroup match> + dst <addr> + dport <port> -> accept
	// This allows the agent (in its cgroup) to connect to the director's NATS
	exprs, err := f.buildCgroupMatchExprs(f.agentCgroup)
	if err != nil {
		return fmt.Errorf("building cgroup match expressions: %w", err)
	}
	exprs = append(exprs, f.buildDestIPExprs(addr)...)
	exprs = append(exprs, f.buildTCPDestPortExprs(port)...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.natsChain,
		Exprs: exprs,
	})

	return nil
}

func (f *NftablesFirewall) addNATSBlockRule(addr net.IP, port int) error {
	// Build rule: dst <addr> + dport <port> -> drop
	// This blocks everyone else (not in agent's cgroup) from connecting to director's NATS.
	// This rule must come AFTER the allow rule so the agent's cgroup is matched first.
	// Note: No cgroup match means this applies to all processes.
	exprs := f.buildDestIPExprs(addr)
	exprs = append(exprs, f.buildTCPDestPortExprs(port)...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictDrop})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.natsChain,
		Exprs: exprs,
	})

	return nil
}

func (f *NftablesFirewall) buildCgroupMatchExprs(cgroup ProcessCgroup) ([]expr.Any, error) {
	if f.cgroupVersion == CgroupV2 {
		// Cgroup v2: match on cgroup ID using socket cgroupv2
		// The nftables "socket cgroupv2" matching compares against the cgroup
		// inode ID (8 bytes), NOT the path string. The nft CLI translates
		// the path to an inode ID at rule add time.
		cgroupID, err := f.cgroupResolver.GetCgroupID(cgroup.Path)
		if err != nil {
			return nil, fmt.Errorf("getting cgroup ID for %s: %w", cgroup.Path, err)
		}

		f.logger.Debug(f.logTag, "Using cgroup v2 socket matching with cgroup ID %d for path %s", cgroupID, cgroup.Path)

		// The cgroup ID is an 8-byte value (uint64) in native byte order
		cgroupIDBytes := make([]byte, 8)
		binary.NativeEndian.PutUint64(cgroupIDBytes, cgroupID)

		return []expr.Any{
			&expr.Socket{
				Key:      expr.SocketKeyCgroupv2,
				Level:    2,
				Register: 1,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     cgroupIDBytes,
			},
		}, nil
	}

	// Cgroup v1: match on classid
	// This matches: meta cgroup <classid>
	classID := cgroup.ClassID
	if classID == 0 {
		// Use default NATS classid if not set
		classID = NATSClassID
	}

	classIDBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(classIDBytes, classID)

	return []expr.Any{
		&expr.Meta{
			Key:      expr.MetaKeyCGROUP,
			Register: 1,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     classIDBytes,
		},
	}, nil
}

func (f *NftablesFirewall) buildLoopbackDestExprs() []expr.Any {
	// Match destination IP 127.0.0.1
	return []expr.Any{
		// Check this is IPv4
		&expr.Meta{
			Key:      expr.MetaKeyNFPROTO,
			Register: 1,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{unix.NFPROTO_IPV4},
		},
		// Load destination IP
		&expr.Payload{
			DestRegister: 1,
			Base:         expr.PayloadBaseNetworkHeader,
			Offset:       16, // Destination IP offset in IPv4 header
			Len:          4,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     net.ParseIP("127.0.0.1").To4(),
		},
	}
}

func (f *NftablesFirewall) buildDestIPExprs(ip net.IP) []expr.Any {
	if ip4 := ip.To4(); ip4 != nil {
		return []expr.Any{
			// Check this is IPv4
			&expr.Meta{
				Key:      expr.MetaKeyNFPROTO,
				Register: 1,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte{unix.NFPROTO_IPV4},
			},
			// Load destination IP
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       16, // Destination IP offset in IPv4 header
				Len:          4,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     ip4,
			},
		}
	}

	// IPv6
	return []expr.Any{
		// Check this is IPv6
		&expr.Meta{
			Key:      expr.MetaKeyNFPROTO,
			Register: 1,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{unix.NFPROTO_IPV6},
		},
		// Load destination IP
		&expr.Payload{
			DestRegister: 1,
			Base:         expr.PayloadBaseNetworkHeader,
			Offset:       24, // Destination IP offset in IPv6 header
			Len:          16,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     ip.To16(),
		},
	}
}

func (f *NftablesFirewall) buildTCPDestPortExprs(port int) []expr.Any {
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))

	return []expr.Any{
		// Check protocol is TCP
		&expr.Meta{
			Key:      expr.MetaKeyL4PROTO,
			Register: 1,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{unix.IPPROTO_TCP},
		},
		// Load destination port
		&expr.Payload{
			DestRegister: 1,
			Base:         expr.PayloadBaseTransportHeader,
			Offset:       2, // Destination port offset in TCP header
			Len:          2,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     portBytes,
		},
	}
}

func (f *NftablesFirewall) buildSetMarkExprs() []expr.Any {
	// Set packet mark to AllowMark (0xb054)
	// This mark is checked by the base bosh_firewall table to skip DROP rules
	markBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(markBytes, AllowMark)

	return []expr.Any{
		// Load mark value into register
		&expr.Immediate{
			Register: 1,
			Data:     markBytes,
		},
		// Set packet mark from register
		&expr.Meta{
			Key:            expr.MetaKeyMARK,
			SourceRegister: true,
			Register:       1,
		},
	}
}

// Helper functions

func policyPtr(p nftables.ChainPolicy) *nftables.ChainPolicy {
	return &p
}

func parseNATSURL(mbusURL string) (string, int, error) {
	// Skip https URLs (create-env case) and empty URLs
	if mbusURL == "" || strings.HasPrefix(mbusURL, "https://") {
		return "", 0, fmt.Errorf("skipping URL: %s", mbusURL)
	}

	// Parse nats://user:pass@host:port format
	u, err := gonetURL.Parse(mbusURL)
	if err != nil {
		return "", 0, err
	}

	if u.Hostname() == "" {
		return "", 0, fmt.Errorf("empty hostname in URL")
	}

	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		// Maybe no port specified, use default NATS port
		host = u.Hostname()
		portStr = "4222"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("parsing port: %w", err)
	}

	if port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("port %d out of valid range (1-65535)", port)
	}

	return host, port, nil
}
