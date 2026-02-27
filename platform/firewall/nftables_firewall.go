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
	"github.com/google/nftables/userdata"
	"golang.org/x/sys/unix"
)

// NftablesConn abstracts the nftables connection for testing
//
//counterfeiter:generate -header ./firewallfakes/linux_build_constraint.txt . NftablesConn
type NftablesConn interface {
	AddTable(t *nftables.Table) *nftables.Table
	AddChain(c *nftables.Chain) *nftables.Chain
	AddRule(r *nftables.Rule) *nftables.Rule
	DelRule(r *nftables.Rule) error
	GetRules(t *nftables.Table, c *nftables.Chain) ([]*nftables.Rule, error)
	ListTables() ([]*nftables.Table, error)
	ListChains() ([]*nftables.Chain, error)
	FlushChain(c *nftables.Chain)
	Flush() error
	CloseLasting() error
}

// DNSResolver abstracts DNS resolution for testing
//
//counterfeiter:generate -header ./firewallfakes/linux_build_constraint.txt . DNSResolver
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
func (r *realNftablesConn) DelRule(rule *nftables.Rule) error {
	return r.conn.DelRule(rule)
}

func (r *realNftablesConn) GetRules(t *nftables.Table, c *nftables.Chain) ([]*nftables.Rule, error) {
	return r.conn.GetRules(t, c)
}

func (r *realNftablesConn) ListTables() ([]*nftables.Table, error) {
	return r.conn.ListTables()
}

func (r *realNftablesConn) ListChains() ([]*nftables.Chain, error) {
	return r.conn.ListChains()
}

func (r *realNftablesConn) FlushChain(c *nftables.Chain) {
	r.conn.FlushChain(c)
}

func (r *realNftablesConn) Flush() error {
	return r.conn.Flush()
}

func (r *realNftablesConn) CloseLasting() error {
	return r.conn.CloseLasting()
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

func (f *NftablesFirewall) EnableMonitAccess() error {
	// 1. Check if jobs chain exists
	err := f.getMonitJobsChainAndTable()
	if err != nil {
		f.logger.Error(f.logTag, "Failed to check if jobs chain exists: %v", err)
		return bosherr.WrapError(err, "Failed to check if jobs chain exists")
	}

	// 2. Try cgroup-based rule first (better isolation)
	cgroupPath, err := getCurrentCgroupPath(f.logger)
	if err == nil && isCgroupAccessible(f.logger, cgroupPath) {
		inodeID, err := getCgroupInodeID(cgroupPath)
		if err == nil {
			f.logger.Info(f.logTag, "Using cgroup rule for: %s (inode: %d)", cgroupPath, inodeID)

			if err := f.addCgroupRule(inodeID, cgroupPath); err == nil {
				f.logger.Info(f.logTag, "Successfully added cgroup-based rule")
				return nil
			} else {
				f.logger.Error(f.logTag, "Failed to add cgroup rule: %v", err)
			}
		} else {
			f.logger.Error(f.logTag, "Failed to get cgroup inode ID: %v", err)
		}
	} else if err != nil {
		f.logger.Error(f.logTag, "Could not detect cgroup: %v", err)
	}

	// 3. Fallback to UID-based rule
	uid := uint32(os.Getuid())
	f.logger.Info(f.logTag, "Falling back to UID rule for UID: %d", uid)

	return f.addUIDRule(uid)
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

func (f *NftablesFirewall) Cleanup() error {
	return f.conn.CloseLasting()
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

// getMonitJobsChainAndTable find the monit jobs table and chain.
// Returns true if both the table "inet bosh_agent" and chain "monit_access_jobs" exist.
func (f *NftablesFirewall) getMonitJobsChainAndTable() error {
	// List all tables to find bosh_agent
	tables, err := f.conn.ListTables()
	if err != nil {
		return bosherr.WrapError(err, "listing tables")
	}

	for _, t := range tables {
		if t.Name == TableName && t.Family == nftables.TableFamilyINet {
			f.table = t
			break
		}
	}
	if f.table == nil {
		return ErrBoshTableNotFound
	}

	// List all chains to find monit_access_jobs
	chains, err := f.conn.ListChains()
	if err != nil {
		return bosherr.WrapError(err, "listing chains")
	}

	for _, c := range chains {
		if c.Table.Name == TableName && c.Name == MonitJobsChainName {
			f.monitJobsChain = c
			break
		}
	}
	if f.monitJobsChain == nil {
		return ErrMonitJobsChainNotFound
	}

	return nil
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
	exprs := buildUIDMatchExprs(0)
	exprs = append(exprs, buildLoopbackDestExprs()...)
	exprs = append(exprs, buildTCPDestPortExprs(MonitPort)...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.monitChain,
		Exprs: exprs,
	})
}

func (f *NftablesFirewall) addMonitBlockRule() {
	// Rule: ip daddr 127.0.0.1 tcp dport 2822 drop
	exprs := buildLoopbackDestExprs()
	exprs = append(exprs, buildTCPDestPortExprs(MonitPort)...)
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
	exprs := buildUIDMatchExprs(0)
	exprs = append(exprs, buildDestIPExprs(addr)...)
	exprs = append(exprs, buildTCPDestPortExprs(port)...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.natsChain,
		Exprs: exprs,
	})
}

func (f *NftablesFirewall) addNATSBlockRule(addr net.IP, port int) {
	// Rule: ip daddr <addr> tcp dport <port> drop
	exprs := buildDestIPExprs(addr)
	exprs = append(exprs, buildTCPDestPortExprs(port)...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictDrop})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.natsChain,
		Exprs: exprs,
	})
}

// addUIDRule adds a UID-based firewall rule to the monit_access_jobs chain.
// This is the fallback when cgroup matching is not available.
func (f *NftablesFirewall) addUIDRule(uid uint32) error {
	// Check if rule already exists (idempotency)
	rules, err := f.conn.GetRules(f.table, f.monitJobsChain)
	if err == nil {
		for _, rule := range rules {
			if ruleMatchesUID(rule, uid) {
				f.logger.Info(f.logTag, "UID rule already exists for UID %d, skipping", uid)
				return nil
			}
		}
	}

	// Build rule expressions:
	// meta skuid <uid> ip daddr 127.0.0.1 tcp dport 2822 log prefix "..." accept
	exprs := buildUIDMatchExprs(uid)
	exprs = append(exprs, buildLoopbackDestExprs()...)
	exprs = append(exprs, buildTCPDestPortExprs(MonitPort)...)
	exprs = append(exprs, buildLogExpr(fmt.Sprintf(MonitAccessLogPrefix+"UID %d match: ", uid))...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})

	f.conn.AddRule(&nftables.Rule{
		Table: f.table,
		Chain: f.monitJobsChain,
		Exprs: exprs,
	})

	if err := f.conn.Flush(); err != nil {
		return fmt.Errorf("flushing nftables rules: %w", err)
	}

	return nil
}

// addCgroupRule adds a cgroup-based firewall rule to the monit_access_jobs chain.
// Uses the cgroup inode ID for matching (required by nftables kernel).
// Tags the rule with the job name extracted from cgroupPath to enable cleanup of stale rules.
func (f *NftablesFirewall) addCgroupRule(inodeID uint64, cgroupPath string) error {
	// Extract job name from cgroup path for tagging
	jobName := extractJobNameFromCgroup(cgroupPath)
	if jobName == "" {
		return fmt.Errorf("could not extract job name from cgroup path: %s", cgroupPath)
	}

	// Clean up any stale cgroup rules for this job before adding new one.
	// This prevents accumulation of rules when cgroups are recreated with new inode IDs.
	// NOTE: cleanupStaleJobRules flushes deletes immediately so subsequent rule checks see the cleaned state.
	if err := f.cleanupStaleJobRules(jobName); err != nil {
		f.logger.Error(f.logTag, "Warning: failed to cleanup stale rules: %v", err)
		// Continue anyway - we'll still add the new rule
	}

	// Check if rule already exists (idempotency)
	// This check happens AFTER cleanup is flushed to ensure we see the current state
	rules, err := f.conn.GetRules(f.table, f.monitJobsChain)
	if err == nil {
		for _, rule := range rules {
			if ruleMatchesCgroup(rule, inodeID) {
				f.logger.Info(f.logTag, "Cgroup rule already exists, skipping")
				return nil
			}
		}
	}

	// Build rule expressions:
	// socket cgroupv2 level 2 <inode-id> ip daddr 127.0.0.1 tcp dport 2822 log prefix "..." accept
	exprs := buildCgroupMatchExprs(inodeID)
	exprs = append(exprs, buildLoopbackDestExprs()...)
	exprs = append(exprs, buildTCPDestPortExprs(MonitPort)...)
	exprs = append(exprs, buildLogExpr(MonitAccessLogPrefix+"cgroup match: ")...)
	exprs = append(exprs, &expr.Verdict{Kind: expr.VerdictAccept})

	// Tag the rule with job name using nftables userdata comment
	// Format: "bosh-monit-access:<job-name>"
	ruleTag := fmt.Sprintf("bosh-monit-access:%s", jobName)
	ruleUserData := userdata.AppendString(nil, userdata.TypeComment, ruleTag)

	f.conn.AddRule(&nftables.Rule{
		Table:    f.table,
		Chain:    f.monitJobsChain,
		Exprs:    exprs,
		UserData: ruleUserData,
	})

	if err := f.conn.Flush(); err != nil {
		return fmt.Errorf("flushing nftables rules: %w", err)
	}

	f.logger.Info(f.logTag, "Added cgroup rule tagged with job '%s'", jobName)
	return nil
}

// buildLoopbackDestExprs creates expressions for matching IPv4 loopback destination.
// Note: IPv6 loopback (::1) is intentionally not protected because monit only
// binds to 127.0.0.1:2822 (see jobsupervisor/monit/provider.go).
func buildLoopbackDestExprs() []expr.Any {
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

func buildDestIPExprs(ip net.IP) []expr.Any {
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

// buildLogExpr creates a log expression with the given prefix.
func buildLogExpr(prefix string) []expr.Any {
	return []expr.Any{
		&expr.Log{
			Key:  1 << unix.NFTA_LOG_PREFIX,
			Data: []byte(prefix),
		},
	}
}

func buildTCPDestPortExprs(port int) []expr.Any {
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

// cleanupStaleJobRules removes all existing rules tagged with the given job name.
// This prevents accumulation of stale rules when cgroups are recreated with new inode IDs.
// Flushes deletes immediately to ensure subsequent rule checks see the cleaned state.
func (f *NftablesFirewall) cleanupStaleJobRules(jobName string) error {
	rules, err := f.conn.GetRules(f.table, f.monitJobsChain)
	if err != nil {
		return fmt.Errorf("getting rules for cleanup: %w", err)
	}

	removedCount := 0
	for _, rule := range rules {
		tag := getRuleJobTag(rule)
		// Delete any rule tagged with our job name
		if tag == jobName {
			if err := f.conn.DelRule(rule); err != nil {
				return fmt.Errorf("deleting stale rule: %w", err)
			}
			removedCount++
		}
	}

	if removedCount > 0 {
		// Flush deletes immediately so subsequent checks see the cleaned state
		if err := f.conn.Flush(); err != nil {
			return fmt.Errorf("flushing rule deletions: %w", err)
		}
		f.logger.Info(f.logTag, "Cleaned up %d stale rule(s) for job '%s'", removedCount, jobName)
	}

	return nil
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

// ruleMatchesUID checks if an existing rule matches the given UID.
func ruleMatchesUID(rule *nftables.Rule, uid uint32) bool {
	hasMetaSKUID := false
	hasUIDMatch := false

	for _, e := range rule.Exprs {
		if metaExpr, ok := e.(*expr.Meta); ok {
			if metaExpr.Key == expr.MetaKeySKUID {
				hasMetaSKUID = true
			}
		}
		if cmpExpr, ok := e.(*expr.Cmp); ok {
			if len(cmpExpr.Data) == 4 {
				existingUID := binary.NativeEndian.Uint32(cmpExpr.Data)
				if existingUID == uid {
					hasUIDMatch = true
				}
			}
		}
	}

	return hasMetaSKUID && hasUIDMatch
}

// ruleMatchesCgroup checks if an existing rule matches the given cgroup inode ID.
func ruleMatchesCgroup(rule *nftables.Rule, inodeID uint64) bool {
	hasSocketExpr := false
	hasCgroupMatch := false

	for _, e := range rule.Exprs {
		if socketExpr, ok := e.(*expr.Socket); ok {
			if socketExpr.Key == expr.SocketKeyCgroupv2 {
				hasSocketExpr = true
			}
		}
		if cmpExpr, ok := e.(*expr.Cmp); ok {
			if len(cmpExpr.Data) == 8 {
				existingID := binary.NativeEndian.Uint64(cmpExpr.Data)
				if existingID == inodeID {
					hasCgroupMatch = true
				}
			}
		}
	}

	return hasSocketExpr && hasCgroupMatch
}

// buildUIDMatchExprs creates expressions for matching socket UID
func buildUIDMatchExprs(uid uint32) []expr.Any {
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

// buildCgroupMatchExprs creates nftables expressions for cgroupv2 socket matching.
// Uses 8-byte inode ID (not path string) as required by kernel.
func buildCgroupMatchExprs(inodeID uint64) []expr.Any {
	// Convert inode ID to 8-byte array in native byte order
	inodeIDBytes := make([]byte, 8)
	binary.NativeEndian.PutUint64(inodeIDBytes, inodeID)

	return []expr.Any{
		&expr.Socket{
			Key:      expr.SocketKeyCgroupv2,
			Level:    2, // Hardcoded level 2 for systemd scope nesting
			Register: 1,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     inodeIDBytes,
		},
	}
}

// extractJobNameFromCgroup extracts the job name from a BPM cgroup path.
// Example: "system.slice/runc-bpm-galera-agent.scope" -> "galera-agent"
func extractJobNameFromCgroup(cgroupPath string) string {
	// BPM cgroups follow pattern: system.slice/runc-bpm-<job-name>.scope
	parts := strings.Split(cgroupPath, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "runc-bpm-") && strings.HasSuffix(part, ".scope") {
			// Extract job name from "runc-bpm-galera-agent.scope"
			jobName := strings.TrimPrefix(part, "runc-bpm-")
			jobName = strings.TrimSuffix(jobName, ".scope")
			return jobName
		}
	}
	return ""
}

// getRuleJobTag retrieves the job name tag from a rule's userdata comment.
func getRuleJobTag(rule *nftables.Rule) string {
	if rule.UserData == nil {
		return ""
	}
	comment, ok := userdata.GetString(rule.UserData, userdata.TypeComment)
	if !ok {
		return ""
	}
	// Our tag format: "bosh-monit-access:<job-name>"
	prefix := "bosh-monit-access:"
	if strings.HasPrefix(comment, prefix) {
		return strings.TrimPrefix(comment, prefix)
	}
	return ""
}
