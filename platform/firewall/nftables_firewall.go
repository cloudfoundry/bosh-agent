//go:build linux

package firewall

import (
	"encoding/binary"
	"fmt"
	"net"
	gonetURL "net/url"
	"strconv"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

const (
	TableName          = "bosh_agent"
	MonitChainName     = "monit_access"
	MonitJobsChainName = "monit_access_jobs"
	NATSChainName      = "nats_access"
	MonitPort          = 2822
)

// NftablesConn abstracts the nftables connection for testing
//
//counterfeiter:generate -header ./linux_header.txt . NftablesConn
type NftablesConn interface {
	AddTable(t *nftables.Table) *nftables.Table
	AddChain(c *nftables.Chain) *nftables.Chain
	AddRule(r *nftables.Rule) *nftables.Rule
	FlushChain(c *nftables.Chain)
	Flush() error
}

// DNSResolver abstracts DNS resolution for testing
//
//counterfeiter:generate -header ./linux_header.txt . DNSResolver
type DNSResolver interface {
	LookupIP(host string) ([]net.IP, error)
}

// realDNSResolver uses the standard library for DNS resolution
type realDNSResolver struct{}

func (r *realDNSResolver) LookupIP(host string) ([]net.IP, error) {
	return net.LookupIP(host)
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

func (r *realNftablesConn) FlushChain(c *nftables.Chain) {
	r.conn.FlushChain(c)
}

func (r *realNftablesConn) Flush() error {
	return r.conn.Flush()
}

// NftablesFirewall implements Manager and NatsFirewallHook using nftables with UID-based matching
type NftablesFirewall struct {
	conn           NftablesConn
	resolver       DNSResolver
	logger         boshlog.Logger
	logTag         string
	table          *nftables.Table
	monitChain     *nftables.Chain
	monitJobsChain *nftables.Chain
	natsChain      *nftables.Chain
}

// NewNftablesFirewall creates a new nftables-based firewall manager
func NewNftablesFirewall(logger boshlog.Logger) (Manager, error) {
	conn, err := nftables.New()
	if err != nil {
		return nil, bosherr.WrapError(err, "Creating nftables connection")
	}

	return NewNftablesFirewallWithDeps(
		&realNftablesConn{conn: conn},
		&realDNSResolver{},
		logger,
	), nil
}

// NewNftablesFirewallWithDeps creates a firewall manager with injected dependencies (for testing)
func NewNftablesFirewallWithDeps(conn NftablesConn, resolver DNSResolver, logger boshlog.Logger) Manager {
	return &NftablesFirewall{
		conn:     conn,
		resolver: resolver,
		logger:   logger,
		logTag:   "NftablesFirewall",
	}
}

// SetupMonitFirewall creates firewall rules to protect monit (port 2822).
// Only root (UID 0) is allowed to connect by default.
// Jobs can add their own access rules to the monit_access_jobs chain.
//
// Architecture:
//   - monit_access_jobs: Regular chain for job-managed rules (never flushed by agent)
//   - monit_access: Base chain with hook that jumps to jobs chain, then applies agent rules
//
// This allows job rules to persist across agent restarts while ensuring
// agent rules are always up-to-date.
func (f *NftablesFirewall) SetupMonitFirewall() error {
	f.logger.Info(f.logTag, "Setting up monit firewall rules (UID-based matching)")

	// Create or get our table
	f.ensureTable()

	// Create jobs chain if it doesn't exist (never flush it - job rules persist)
	f.ensureMonitJobsChain()

	// Create monit chain
	f.ensureMonitChain()

	// Flush existing agent rules to ensure idempotency on restart
	f.conn.FlushChain(f.monitChain)

	// Add jump to jobs chain first (so job rules are checked before agent rules)
	f.addJumpToJobsChain()

	// Add allow rule for root (UID 0)
	f.addMonitAllowRule()

	// Add block rule for everyone else
	f.addMonitBlockRule()

	// Commit all rules
	if err := f.conn.Flush(); err != nil {
		return bosherr.WrapError(err, "Flushing nftables rules")
	}

	f.logger.Info(f.logTag, "Successfully set up monit firewall rules")
	return nil
}

// SetupNATSFirewall creates firewall rules to protect NATS.
// This resolves DNS and should be called before each connection attempt.
func (f *NftablesFirewall) SetupNATSFirewall(mbusURL string) error {
	// Parse URL to get host and port
	host, port, err := parseNATSURL(mbusURL)
	if err != nil {
		// Not an error for https URLs or empty URLs (create-env case)
		f.logger.Info(f.logTag, "Skipping NATS firewall: %s", err)
		return nil
	}

	// Resolve host to IP addresses
	var addrs []net.IP
	if ip := net.ParseIP(host); ip != nil {
		addrs = []net.IP{ip}
	} else {
		addrs, err = f.resolver.LookupIP(host)
		if err != nil {
			f.logger.Warn(f.logTag, "DNS resolution failed for %s: %s", host, err)
			return nil
		}
	}

	f.logger.Debug(f.logTag, "Setting up NATS firewall for %s:%d (resolved to %v)", host, port, addrs)

	// Ensure table exists
	f.ensureTable()

	// Ensure NATS chain exists
	f.ensureNATSChain()

	// Flush NATS chain (removes old rules for previous IPs)
	f.conn.FlushChain(f.natsChain)

	// Add rules for each resolved IP
	for _, addr := range addrs {
		f.addNATSAllowRule(addr, port)
		f.addNATSBlockRule(addr, port)
	}

	// Commit
	if err := f.conn.Flush(); err != nil {
		return bosherr.WrapError(err, "Flushing nftables rules")
	}

	f.logger.Info(f.logTag, "Updated NATS firewall rules for %s:%d", host, port)
	return nil
}

// BeforeConnect implements NatsFirewallHook. Called before each NATS connection attempt.
func (f *NftablesFirewall) BeforeConnect(mbusURL string) error {
	return f.SetupNATSFirewall(mbusURL)
}

func (f *NftablesFirewall) ensureTable() {
	f.table = &nftables.Table{
		Family: nftables.TableFamilyINet,
		Name:   TableName,
	}
	f.conn.AddTable(f.table)
}

func (f *NftablesFirewall) ensureMonitChain() {
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
}

// ensureMonitJobsChain creates a regular chain (no hook) for job-managed rules.
// This chain is never flushed by the agent, allowing job rules to persist across agent restarts.
// Jobs can add rules to this chain via pre-start scripts using the nft CLI.
func (f *NftablesFirewall) ensureMonitJobsChain() {
	f.monitJobsChain = &nftables.Chain{
		Name:  MonitJobsChainName,
		Table: f.table,
		// No Type, Hooknum, Priority, or Policy - this is a regular chain
		// that can only be reached via jump from monit_access
	}
	f.conn.AddChain(f.monitJobsChain)
}

func (f *NftablesFirewall) ensureNATSChain() {
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
}

func (f *NftablesFirewall) addMonitAllowRule() {
	// Rule: meta skuid 0 ip daddr 127.0.0.1 tcp dport 2822 accept
	exprs := f.buildUIDMatchExprs(0)
	exprs = append(exprs, f.buildLoopbackDestExprs()...)
	exprs = append(exprs, f.buildTCPDestPortExprs(MonitPort)...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.monitChain,
		Exprs: exprs,
	})
}

func (f *NftablesFirewall) addMonitBlockRule() {
	// Rule: ip daddr 127.0.0.1 tcp dport 2822 drop
	exprs := f.buildLoopbackDestExprs()
	exprs = append(exprs, f.buildTCPDestPortExprs(MonitPort)...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictDrop})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.monitChain,
		Exprs: exprs,
	})
}

// addJumpToJobsChain adds a jump rule to the monit_access_jobs chain.
// This must be the first rule in monit_access so job rules are evaluated first.
func (f *NftablesFirewall) addJumpToJobsChain() {
	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.monitChain,
		Exprs: []expr.Any{
			&expr.Verdict{
				Kind:  expr.VerdictJump,
				Chain: MonitJobsChainName,
			},
		},
	})
}

func (f *NftablesFirewall) addNATSAllowRule(addr net.IP, port int) {
	// Rule: meta skuid 0 ip daddr <addr> tcp dport <port> accept
	exprs := f.buildUIDMatchExprs(0)
	exprs = append(exprs, f.buildDestIPExprs(addr)...)
	exprs = append(exprs, f.buildTCPDestPortExprs(port)...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.natsChain,
		Exprs: exprs,
	})
}

func (f *NftablesFirewall) addNATSBlockRule(addr net.IP, port int) {
	// Rule: ip daddr <addr> tcp dport <port> drop
	exprs := f.buildDestIPExprs(addr)
	exprs = append(exprs, f.buildTCPDestPortExprs(port)...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictDrop})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.natsChain,
		Exprs: exprs,
	})
}

// buildUIDMatchExprs creates expressions for matching socket UID
func (f *NftablesFirewall) buildUIDMatchExprs(uid uint32) []expr.Any {
	uidBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(uidBytes, uid)

	return []expr.Any{
		&expr.Meta{
			Key:      expr.MetaKeySKUID,
			Register: 1,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     uidBytes,
		},
	}
}

// buildLoopbackDestExprs creates expressions for matching IPv4 loopback destination.
// Note: IPv6 loopback (::1) is intentionally not protected because monit only
// binds to 127.0.0.1:2822 (see jobsupervisor/monit/provider.go).
func (f *NftablesFirewall) buildLoopbackDestExprs() []expr.Any {
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
			&expr.Meta{
				Key:      expr.MetaKeyNFPROTO,
				Register: 1,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte{unix.NFPROTO_IPV4},
			},
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       16,
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
		&expr.Meta{
			Key:      expr.MetaKeyNFPROTO,
			Register: 1,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{unix.NFPROTO_IPV6},
		},
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
		&expr.Meta{
			Key:      expr.MetaKeyL4PROTO,
			Register: 1,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{unix.IPPROTO_TCP},
		},
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

func policyPtr(p nftables.ChainPolicy) *nftables.ChainPolicy {
	return &p
}

func parseNATSURL(mbusURL string) (string, int, error) {
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

	if port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("port %d out of valid range (1-65535)", port)
	}

	return host, port, nil
}
