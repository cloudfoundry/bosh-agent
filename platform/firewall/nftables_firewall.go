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

	TableName = "bosh_agent"
	ChainName = "agent_exceptions"

	MonitPort = 2822
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// NftablesConn abstracts the nftables connection for testing
//
//counterfeiter:generate . NftablesConn
type NftablesConn interface {
	AddTable(t *nftables.Table) *nftables.Table
	AddChain(c *nftables.Chain) *nftables.Chain
	AddRule(r *nftables.Rule) *nftables.Rule
	DelTable(t *nftables.Table)
	Flush() error
}

// CgroupResolver abstracts cgroup detection for testing
//
//counterfeiter:generate . CgroupResolver
type CgroupResolver interface {
	DetectVersion() (CgroupVersion, error)
	GetProcessCgroup(pid int, version CgroupVersion) (ProcessCgroup, error)
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

// NftablesFirewall implements Manager using nftables via netlink
type NftablesFirewall struct {
	conn           NftablesConn
	cgroupResolver CgroupResolver
	cgroupVersion  CgroupVersion
	logger         boshlog.Logger
	logTag         string
	table          *nftables.Table
	chain          *nftables.Chain
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

// SetupAgentRules sets up the agent's own firewall exceptions during bootstrap
func (f *NftablesFirewall) SetupAgentRules(mbusURL string, enableNATSFirewall bool) error {
	f.logger.Info(f.logTag, "Setting up agent firewall rules (enableNATSFirewall=%v)", enableNATSFirewall)

	// Create or get our table
	if err := f.ensureTable(); err != nil {
		return bosherr.WrapError(err, "Creating nftables table")
	}

	// Create our chain with priority -1 (runs before base rules at priority 0)
	if err := f.ensureChain(); err != nil {
		return bosherr.WrapError(err, "Creating nftables chain")
	}

	// Get agent's own cgroup path/classid
	agentCgroup, err := f.cgroupResolver.GetProcessCgroup(os.Getpid(), f.cgroupVersion)
	if err != nil {
		return bosherr.WrapError(err, "Getting agent cgroup")
	}

	f.logger.Debug(f.logTag, "Agent cgroup: version=%d path=%s classid=%d",
		agentCgroup.Version, agentCgroup.Path, agentCgroup.ClassID)

	// Add rule: agent can access monit
	if err := f.addMonitRule(agentCgroup); err != nil {
		return bosherr.WrapError(err, "Adding agent monit rule")
	}

	// Add NATS rules only if enabled (Jammy: true, Noble: false)
	if enableNATSFirewall && mbusURL != "" {
		if err := f.addNATSRules(agentCgroup, mbusURL); err != nil {
			return bosherr.WrapError(err, "Adding agent NATS rules")
		}
	}

	// Commit all rules
	if err := f.conn.Flush(); err != nil {
		return bosherr.WrapError(err, "Flushing nftables rules")
	}

	f.logger.Info(f.logTag, "Successfully set up firewall rules")
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
	if err := f.ensureChain(); err != nil {
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

func (f *NftablesFirewall) ensureChain() error {
	// Priority -1 ensures our ACCEPT rules run before base DROP rules (priority 0)
	priority := nftables.ChainPriority(*nftables.ChainPriorityFilter - 1)

	f.chain = &nftables.Chain{
		Name:     ChainName,
		Table:    f.table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookOutput,
		Priority: &priority,
		Policy:   policyPtr(nftables.ChainPolicyAccept),
	}
	f.conn.AddChain(f.chain)
	return nil
}

func (f *NftablesFirewall) addMonitRule(cgroup ProcessCgroup) error {
	// Build rule: <cgroup match> + dst 127.0.0.1 + dport 2822 -> accept
	exprs := f.buildCgroupMatchExprs(cgroup)
	exprs = append(exprs, f.buildLoopbackDestExprs()...)
	exprs = append(exprs, f.buildTCPDestPortExprs(MonitPort)...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.chain,
		Exprs: exprs,
	})

	return nil
}

func (f *NftablesFirewall) addNATSRules(cgroup ProcessCgroup, mbusURL string) error {
	// Parse NATS URL to get host and port
	host, port, err := parseNATSURL(mbusURL)
	if err != nil {
		// Not an error for https URLs or empty URLs
		f.logger.Debug(f.logTag, "Skipping NATS firewall: %s", err)
		return nil
	}

	// Resolve host to IP addresses
	addrs, err := net.LookupIP(host)
	if err != nil {
		return bosherr.WrapError(err, fmt.Sprintf("Resolving NATS host: %s", host))
	}

	for _, addr := range addrs {
		exprs := f.buildCgroupMatchExprs(cgroup)
		exprs = append(exprs, f.buildDestIPExprs(addr)...)
		exprs = append(exprs, f.buildTCPDestPortExprs(port)...)
		exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})

		f.conn.AddRule(&nftables.Rule{
			Table: f.table,
			Chain: f.chain,
			Exprs: exprs,
		})
	}

	f.logger.Info(f.logTag, "Added NATS firewall rules for %s:%d", host, port)
	return nil
}

func (f *NftablesFirewall) buildCgroupMatchExprs(cgroup ProcessCgroup) []expr.Any {
	if f.cgroupVersion == CgroupV2 {
		// Cgroup v2: match on cgroup path using socket expression
		// This matches: socket cgroupv2 level 2 "<path>"
		return []expr.Any{
			&expr.Socket{
				Key:      expr.SocketKeyCgroupv2,
				Level:    2,
				Register: 1,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte(cgroup.Path + "\x00"),
			},
		}
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
	}
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

	return host, port, nil
}
