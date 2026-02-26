//go:build linux

package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

func main() {
	tableName := flag.String("table", "", "nftables table name")
	chainName := flag.String("chain", "", "nftables chain name")
	flush := flag.Bool("flush", false, "flush (delete all rules from) the chain instead of listing")
	flag.Parse()

	if *tableName == "" || *chainName == "" {
		fmt.Fprintln(os.Stderr, "usage: nftables-checker --table TABLE --chain CHAIN [--flush]")
		os.Exit(2)
	}

	conn, err := nftables.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open nftables connection: %v\n", err)
		os.Exit(1)
	}

	table, chain, err := findChain(conn, *tableName, *chainName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if *flush {
		conn.FlushChain(chain)
		if err := conn.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to flush chain: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("flushed chain %s in table %s\n", *chainName, *tableName)
		return
	}

	rules, err := conn.GetRules(table, chain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get rules: %v\n", err)
		os.Exit(1)
	}

	if len(rules) == 0 {
		fmt.Println("no rules found")
		os.Exit(0)
	}

	for _, rule := range rules {
		fmt.Println(formatRule(rule))
	}
}

func findChain(conn *nftables.Conn, tableName, chainName string) (*nftables.Table, *nftables.Chain, error) {
	tables, err := conn.ListTables()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list tables: %w", err)
	}

	var table *nftables.Table
	for _, t := range tables {
		if t.Name == tableName && t.Family == nftables.TableFamilyINet {
			table = t
			break
		}
	}
	if table == nil {
		return nil, nil, fmt.Errorf("table inet %s not found", tableName)
	}

	chains, err := conn.ListChains()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list chains: %w", err)
	}

	for _, c := range chains {
		if c.Table.Name == tableName && c.Name == chainName {
			return table, c, nil
		}
	}
	return nil, nil, fmt.Errorf("chain %s not found in table %s", chainName, tableName)
}

func formatRule(rule *nftables.Rule) string {
	var verdict string
	var uid = -1
	var dstIP net.IP

	for _, e := range rule.Exprs {
		switch v := e.(type) {
		case *expr.Verdict:
			switch v.Kind {
			case expr.VerdictAccept:
				verdict = "ACCEPT"
			case expr.VerdictDrop:
				verdict = "DROP"
			default:
				verdict = fmt.Sprintf("verdict=%d", v.Kind)
			}
		case *expr.Meta:
			if v.Key == expr.MetaKeySKUID {
				uid = extractUID(rule)
			}
		}
	}

	dstIP = extractDestIP(rule)
	dport := extractDestPort(rule)

	result := verdict
	if uid >= 0 {
		result += fmt.Sprintf(" uid=%d", uid)
	}
	if dstIP != nil {
		result += fmt.Sprintf(" dst=%s", dstIP)
	}
	if dport >= 0 {
		result += fmt.Sprintf(" dport=%d", dport)
	}
	return result
}

func extractUID(rule *nftables.Rule) int {
	foundSKUID := false
	for _, e := range rule.Exprs {
		switch v := e.(type) {
		case *expr.Meta:
			if v.Key == expr.MetaKeySKUID {
				foundSKUID = true
			}
		case *expr.Cmp:
			if foundSKUID && len(v.Data) == 4 {
				return int(binary.NativeEndian.Uint32(v.Data))
			}
			foundSKUID = false
		default:
			foundSKUID = false
		}
	}
	return -1
}

func extractDestIP(rule *nftables.Rule) net.IP {
	foundNFProto := false
	var proto byte

	for _, e := range rule.Exprs {
		switch v := e.(type) {
		case *expr.Meta:
			if v.Key == expr.MetaKeyNFPROTO {
				foundNFProto = true
			}
		case *expr.Cmp:
			if foundNFProto && len(v.Data) == 1 {
				proto = v.Data[0]
				foundNFProto = false
				continue
			}
			// After a Payload load, the next Cmp holds the IP
			if proto == unix.NFPROTO_IPV4 && len(v.Data) == 4 {
				return net.IP(v.Data)
			}
			if proto == unix.NFPROTO_IPV6 && len(v.Data) == 16 {
				return net.IP(v.Data)
			}
		case *expr.Payload:
			// IPv4 dst offset=16 len=4, IPv6 dst offset=24 len=16
			if v.Base == expr.PayloadBaseNetworkHeader &&
				((v.Offset == 16 && v.Len == 4) || (v.Offset == 24 && v.Len == 16)) {
				continue
			}
			proto = 0
		default:
			foundNFProto = false
		}
	}
	return nil
}

func extractDestPort(rule *nftables.Rule) int {
	foundTCP := false
	foundPayload := false

	for _, e := range rule.Exprs {
		switch v := e.(type) {
		case *expr.Meta:
			if v.Key == expr.MetaKeyL4PROTO {
				foundTCP = false
				foundPayload = false
			}
		case *expr.Cmp:
			if !foundTCP && len(v.Data) == 1 && v.Data[0] == unix.IPPROTO_TCP {
				foundTCP = true
				continue
			}
			if foundTCP && foundPayload && len(v.Data) == 2 {
				return int(binary.BigEndian.Uint16(v.Data))
			}
			foundPayload = false
		case *expr.Payload:
			if foundTCP && v.Base == expr.PayloadBaseTransportHeader && v.Offset == 2 && v.Len == 2 {
				foundPayload = true
				continue
			}
			foundPayload = false
		default:
			if foundTCP {
				continue
			}
		}
	}
	return -1
}
