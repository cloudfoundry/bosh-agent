//go:build linux

// nft-dump is a minimal utility that uses the nftables Go library to inspect firewall rules.
// It outputs human-readable YAML with interpreted values (IP addresses, ports, etc.).
//
// Usage:
//
//	nft-dump check                    - exit 0 if nftables kernel support exists
//	nft-dump tables                   - list all tables
//	nft-dump table <family> <name>    - dump a specific table (e.g., "inet bosh_agent")
//	nft-dump delete <family> <name>   - delete a specific table
package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "check":
		cmdCheck()
	case "tables":
		cmdTables()
	case "table":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: nft-dump table <family> <name>\n")
			os.Exit(1)
		}
		cmdTable(os.Args[2], os.Args[3])
	case "delete":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: nft-dump delete <family> <name>\n")
			os.Exit(1)
		}
		cmdDelete(os.Args[2], os.Args[3])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `nft-dump - nftables inspection utility using Go netlink library

Usage:
  nft-dump check                    Check if nftables kernel support exists (exit 0 = yes)
  nft-dump tables                   List all tables
  nft-dump table <family> <name>    Dump a specific table (e.g., "inet bosh_agent")
  nft-dump delete <family> <name>   Delete a specific table
  nft-dump help                     Show this help

Families: inet, ip, ip6, arp, bridge, netdev
`)
}

// cmdCheck verifies nftables kernel support exists
func cmdCheck() {
	conn, err := nftables.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nftables not available: %v\n", err)
		os.Exit(1)
	}

	// Try to list tables - this will fail if kernel doesn't support nftables
	_, err = conn.ListTables()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nftables kernel support not available: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("nftables kernel support available")
	os.Exit(0)
}

// TableInfo represents a table in YAML output
type TableInfo struct {
	Family string `yaml:"family"`
	Name   string `yaml:"name"`
}

// cmdTables lists all nftables tables
func cmdTables() {
	conn, err := nftables.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to nftables: %v\n", err)
		os.Exit(1)
	}

	tables, err := conn.ListTables()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list tables: %v\n", err)
		os.Exit(1)
	}

	var output struct {
		Tables []TableInfo `yaml:"tables"`
	}

	for _, t := range tables {
		output.Tables = append(output.Tables, TableInfo{
			Family: familyToString(t.Family),
			Name:   t.Name,
		})
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal YAML: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(string(data))
}

// ChainInfo represents a chain in YAML output
type ChainInfo struct {
	Name     string     `yaml:"name"`
	Type     string     `yaml:"type,omitempty"`
	Hook     string     `yaml:"hook,omitempty"`
	Priority int        `yaml:"priority,omitempty"`
	Policy   string     `yaml:"policy,omitempty"`
	Rules    []RuleInfo `yaml:"rules,omitempty"`
}

// RuleInfo represents a rule in YAML output
type RuleInfo struct {
	Handle  uint64 `yaml:"handle"`
	Summary string `yaml:"summary"` // Human-readable summary of what the rule does
	Match   string `yaml:"match"`   // What the rule matches (cgroup, ip, port, etc.)
	Action  string `yaml:"action"`  // What happens when matched (accept, drop, mark, etc.)
}

// TableDump represents the full dump of a table
type TableDump struct {
	Table  TableInfo   `yaml:"table"`
	Chains []ChainInfo `yaml:"chains"`
}

// ruleAnalyzer accumulates state while analyzing rule expressions
type ruleAnalyzer struct {
	matchType   string // "cgroupv2", "cgroup", "skuid", "skgid", etc.
	matchValue  string // The matched value (cgroup ID, UID, etc.)
	protocol    string // "tcp", "udp", "icmp", etc.
	family      string // "ipv4", "ipv6"
	srcIP       string
	dstIP       string
	srcPort     uint16
	dstPort     uint16
	setMark     uint32
	hasSetMark  bool
	verdict     string // "accept", "drop", "return", etc.
	jumpTarget  string
	counter     bool
	log         string
	expressions []expr.Any
}

// cmdTable dumps a specific table
func cmdTable(familyStr, name string) {
	family, err := parseFamily(familyStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid family '%s': %v\n", familyStr, err)
		os.Exit(1)
	}

	conn, err := nftables.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to nftables: %v\n", err)
		os.Exit(1)
	}

	// Find the table
	tables, err := conn.ListTablesOfFamily(family)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list tables: %v\n", err)
		os.Exit(1)
	}

	var table *nftables.Table
	for _, t := range tables {
		if t.Name == name {
			table = t
			break
		}
	}

	if table == nil {
		fmt.Fprintf(os.Stderr, "Table '%s %s' not found\n", familyStr, name)
		os.Exit(1)
	}

	// Get all chains for this table
	allChains, err := conn.ListChainsOfTableFamily(family)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list chains: %v\n", err)
		os.Exit(1)
	}

	// Filter chains for our table
	var tableChains []*nftables.Chain
	for _, c := range allChains {
		if c.Table.Name == name {
			tableChains = append(tableChains, c)
		}
	}

	// Build output
	output := TableDump{
		Table: TableInfo{
			Family: familyToString(table.Family),
			Name:   table.Name,
		},
	}

	for _, chain := range tableChains {
		chainInfo := ChainInfo{
			Name: chain.Name,
		}

		if chain.Type != "" {
			chainInfo.Type = string(chain.Type)
		}
		if chain.Hooknum != nil {
			chainInfo.Hook = hookToString(*chain.Hooknum)
		}
		if chain.Priority != nil {
			chainInfo.Priority = int(*chain.Priority)
		}
		if chain.Policy != nil {
			chainInfo.Policy = policyToString(*chain.Policy)
		}

		// Get rules for this chain
		rules, err := conn.GetRules(table, chain)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get rules for chain %s: %v\n", chain.Name, err)
		} else {
			for _, rule := range rules {
				ruleInfo := analyzeRule(rule)
				chainInfo.Rules = append(chainInfo.Rules, ruleInfo)
			}
		}

		output.Chains = append(output.Chains, chainInfo)
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal YAML: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(string(data))
}

// analyzeRule extracts meaningful information from a rule's expressions
func analyzeRule(rule *nftables.Rule) RuleInfo {
	ra := &ruleAnalyzer{
		expressions: rule.Exprs,
	}

	// Analyze expressions to extract semantic meaning
	for i := 0; i < len(rule.Exprs); i++ {
		e := rule.Exprs[i]

		switch v := e.(type) {
		case *expr.Socket:
			switch v.Key {
			case expr.SocketKeyCgroupv2:
				ra.matchType = "cgroupv2"
				// The cgroup ID will be in the next Cmp expression
				if i+1 < len(rule.Exprs) {
					if cmp, ok := rule.Exprs[i+1].(*expr.Cmp); ok {
						ra.matchValue = fmt.Sprintf("cgroup_id=%d", parseCgroupID(cmp.Data))
					}
				}
			case expr.SocketKeyMark:
				ra.matchType = "socket_mark"
			case expr.SocketKeyTransparent:
				ra.matchType = "socket_transparent"
			case expr.SocketKeyWildcard:
				ra.matchType = "socket_wildcard"
			}

		case *expr.Meta:
			switch v.Key {
			case expr.MetaKeyCGROUP:
				ra.matchType = "cgroup"
				// The cgroup classid will be in the next Cmp expression
				if i+1 < len(rule.Exprs) {
					if cmp, ok := rule.Exprs[i+1].(*expr.Cmp); ok {
						ra.matchValue = fmt.Sprintf("classid=0x%x", binary.BigEndian.Uint32(padToLength(cmp.Data, 4)))
					}
				}
			case expr.MetaKeySKUID:
				ra.matchType = "skuid"
				if i+1 < len(rule.Exprs) {
					if cmp, ok := rule.Exprs[i+1].(*expr.Cmp); ok {
						ra.matchValue = fmt.Sprintf("uid=%d", binary.LittleEndian.Uint32(padToLength(cmp.Data, 4)))
					}
				}
			case expr.MetaKeySKGID:
				ra.matchType = "skgid"
				if i+1 < len(rule.Exprs) {
					if cmp, ok := rule.Exprs[i+1].(*expr.Cmp); ok {
						ra.matchValue = fmt.Sprintf("gid=%d", binary.LittleEndian.Uint32(padToLength(cmp.Data, 4)))
					}
				}
			case expr.MetaKeyNFPROTO:
				if i+1 < len(rule.Exprs) {
					if cmp, ok := rule.Exprs[i+1].(*expr.Cmp); ok && len(cmp.Data) > 0 {
						switch cmp.Data[0] {
						case 2: // NFPROTO_IPV4
							ra.family = "ipv4"
						case 10: // NFPROTO_IPV6
							ra.family = "ipv6"
						}
					}
				}
			case expr.MetaKeyL4PROTO:
				if i+1 < len(rule.Exprs) {
					if cmp, ok := rule.Exprs[i+1].(*expr.Cmp); ok && len(cmp.Data) > 0 {
						switch cmp.Data[0] {
						case 6:
							ra.protocol = "tcp"
						case 17:
							ra.protocol = "udp"
						case 1:
							ra.protocol = "icmp"
						case 58:
							ra.protocol = "icmpv6"
						}
					}
				}
			case expr.MetaKeyMARK:
				// Could be reading mark for comparison or setting mark
				// Check if this is a set operation (SourceRegister=true means we're setting)
				if v.SourceRegister {
					// Mark is being set - the value should have been loaded by a previous Immediate
					// We handle this in the Immediate case
				}
			}

		case *expr.Payload:
			// Check what comes after to interpret the payload
			if i+1 < len(rule.Exprs) {
				if cmp, ok := rule.Exprs[i+1].(*expr.Cmp); ok {
					switch {
					case v.Base == expr.PayloadBaseNetworkHeader && v.Offset == 12 && v.Len == 4:
						// IPv4 source address
						ra.srcIP = formatIPv4(cmp.Data)
					case v.Base == expr.PayloadBaseNetworkHeader && v.Offset == 16 && v.Len == 4:
						// IPv4 destination address
						ra.dstIP = formatIPv4(cmp.Data)
					case v.Base == expr.PayloadBaseNetworkHeader && v.Offset == 8 && v.Len == 16:
						// IPv6 source address
						ra.srcIP = formatIPv6(cmp.Data)
					case v.Base == expr.PayloadBaseNetworkHeader && v.Offset == 24 && v.Len == 16:
						// IPv6 destination address
						ra.dstIP = formatIPv6(cmp.Data)
					case v.Base == expr.PayloadBaseTransportHeader && v.Offset == 0 && v.Len == 2:
						// Source port
						ra.srcPort = binary.BigEndian.Uint16(padToLength(cmp.Data, 2))
					case v.Base == expr.PayloadBaseTransportHeader && v.Offset == 2 && v.Len == 2:
						// Destination port
						ra.dstPort = binary.BigEndian.Uint16(padToLength(cmp.Data, 2))
					}
				}
			}

		case *expr.Immediate:
			// Check if this is setting a mark - look for a following Meta with SourceRegister
			if v.Register == 1 && len(v.Data) == 4 {
				for j := i + 1; j < len(rule.Exprs); j++ {
					if meta, ok := rule.Exprs[j].(*expr.Meta); ok && meta.Key == expr.MetaKeyMARK && meta.SourceRegister {
						ra.setMark = binary.LittleEndian.Uint32(v.Data)
						ra.hasSetMark = true
						break
					}
				}
			}

		case *expr.Verdict:
			ra.verdict = verdictKindToString(v.Kind)
			if v.Chain != "" {
				ra.jumpTarget = v.Chain
			}

		case *expr.Counter:
			ra.counter = true

		case *expr.Log:
			if len(v.Data) > 0 {
				ra.log = string(v.Data)
			}
		}
	}

	return ra.toRuleInfo(rule.Handle)
}

// toRuleInfo converts the analyzed rule to a RuleInfo struct
func (ra *ruleAnalyzer) toRuleInfo(handle uint64) RuleInfo {
	info := RuleInfo{
		Handle: handle,
	}

	// Build match description
	var matchParts []string

	if ra.matchType != "" {
		if ra.matchValue != "" {
			matchParts = append(matchParts, fmt.Sprintf("%s %s", ra.matchType, ra.matchValue))
		} else {
			matchParts = append(matchParts, ra.matchType)
		}
	}

	if ra.family != "" {
		matchParts = append(matchParts, ra.family)
	}

	if ra.protocol != "" {
		matchParts = append(matchParts, ra.protocol)
	}

	if ra.srcIP != "" {
		matchParts = append(matchParts, fmt.Sprintf("saddr %s", ra.srcIP))
	}

	if ra.dstIP != "" {
		matchParts = append(matchParts, fmt.Sprintf("daddr %s", ra.dstIP))
	}

	if ra.srcPort != 0 {
		matchParts = append(matchParts, fmt.Sprintf("sport %d", ra.srcPort))
	}

	if ra.dstPort != 0 {
		matchParts = append(matchParts, fmt.Sprintf("dport %d", ra.dstPort))
	}

	if len(matchParts) > 0 {
		info.Match = strings.Join(matchParts, " ")
	} else {
		info.Match = "(all)"
	}

	// Build action description
	var actionParts []string

	if ra.counter {
		actionParts = append(actionParts, "counter")
	}

	if ra.log != "" {
		actionParts = append(actionParts, fmt.Sprintf("log prefix %q", ra.log))
	}

	if ra.hasSetMark {
		actionParts = append(actionParts, fmt.Sprintf("mark set 0x%x", ra.setMark))
	}

	if ra.verdict != "" {
		if ra.jumpTarget != "" {
			actionParts = append(actionParts, fmt.Sprintf("%s %s", ra.verdict, ra.jumpTarget))
		} else {
			actionParts = append(actionParts, ra.verdict)
		}
	}

	if len(actionParts) > 0 {
		info.Action = strings.Join(actionParts, " ")
	} else {
		info.Action = "(none)"
	}

	// Build summary
	info.Summary = fmt.Sprintf("%s -> %s", info.Match, info.Action)

	return info
}

// cmdDelete deletes a specific table
func cmdDelete(familyStr, name string) {
	family, err := parseFamily(familyStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid family '%s': %v\n", familyStr, err)
		os.Exit(1)
	}

	conn, err := nftables.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to nftables: %v\n", err)
		os.Exit(1)
	}

	// Delete the table
	conn.DelTable(&nftables.Table{
		Family: family,
		Name:   name,
	})

	if err := conn.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to delete table: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted table %s %s\n", familyStr, name)
}

// Helper functions

func parseFamily(s string) (nftables.TableFamily, error) {
	switch strings.ToLower(s) {
	case "inet":
		return nftables.TableFamilyINet, nil
	case "ip", "ip4", "ipv4":
		return nftables.TableFamilyIPv4, nil
	case "ip6", "ipv6":
		return nftables.TableFamilyIPv6, nil
	case "arp":
		return nftables.TableFamilyARP, nil
	case "bridge":
		return nftables.TableFamilyBridge, nil
	case "netdev":
		return nftables.TableFamilyNetdev, nil
	default:
		return 0, fmt.Errorf("unknown family: %s", s)
	}
}

func familyToString(f nftables.TableFamily) string {
	switch f {
	case nftables.TableFamilyINet:
		return "inet"
	case nftables.TableFamilyIPv4:
		return "ip"
	case nftables.TableFamilyIPv6:
		return "ip6"
	case nftables.TableFamilyARP:
		return "arp"
	case nftables.TableFamilyBridge:
		return "bridge"
	case nftables.TableFamilyNetdev:
		return "netdev"
	default:
		return fmt.Sprintf("unknown(%d)", f)
	}
}

func hookToString(h nftables.ChainHook) string {
	switch h {
	case *nftables.ChainHookPrerouting:
		return "prerouting"
	case *nftables.ChainHookInput:
		return "input"
	case *nftables.ChainHookForward:
		return "forward"
	case *nftables.ChainHookOutput:
		return "output"
	case *nftables.ChainHookPostrouting:
		return "postrouting"
	case *nftables.ChainHookIngress:
		return "ingress"
	default:
		return fmt.Sprintf("unknown(%d)", h)
	}
}

func policyToString(p nftables.ChainPolicy) string {
	switch p {
	case nftables.ChainPolicyAccept:
		return "accept"
	case nftables.ChainPolicyDrop:
		return "drop"
	default:
		return fmt.Sprintf("unknown(%d)", p)
	}
}

func verdictKindToString(k expr.VerdictKind) string {
	switch k {
	case expr.VerdictAccept:
		return "accept"
	case expr.VerdictDrop:
		return "drop"
	case expr.VerdictReturn:
		return "return"
	case expr.VerdictJump:
		return "jump"
	case expr.VerdictGoto:
		return "goto"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// parseCgroupID extracts a cgroup ID from comparison data (little-endian uint64)
func parseCgroupID(data []byte) uint64 {
	if len(data) >= 8 {
		return binary.LittleEndian.Uint64(data)
	}
	if len(data) >= 4 {
		return uint64(binary.LittleEndian.Uint32(data))
	}
	return 0
}

// formatIPv4 formats a 4-byte slice as an IPv4 address
func formatIPv4(data []byte) string {
	if len(data) >= 4 {
		return net.IP(data[:4]).String()
	}
	return fmt.Sprintf("(invalid: %v)", data)
}

// formatIPv6 formats a 16-byte slice as an IPv6 address
func formatIPv6(data []byte) string {
	if len(data) >= 16 {
		return net.IP(data[:16]).String()
	}
	return fmt.Sprintf("(invalid: %v)", data)
}

// padToLength pads a byte slice to the specified length
func padToLength(data []byte, length int) []byte {
	if len(data) >= length {
		return data[:length]
	}
	result := make([]byte, length)
	copy(result[length-len(data):], data)
	return result
}
