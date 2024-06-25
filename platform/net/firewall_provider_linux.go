//go:build linux
// +build linux

package net

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	gonetURL "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	// NOTE: "cgroups is only intended to be used/compiled on linux based system"
	// see: https://github.com/containerd/cgroups/issues/19
	"github.com/containerd/cgroups"
	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/coreos/go-iptables/iptables"
	"github.com/coreos/go-systemd/v22/dbus"
)

const (
	/* "natsIsolationClassID" This is the integer value of the argument "0xb0540002", which is
	   b054:0002 . The major number (the left-hand side) is "BOSH", leet-ified.
	   The minor number (the right-hand side) is 2, indicating that this is the
	   second thing in our "BOSH" classid namespace.

	   _Hopefully_ noone uses a major number of "b054", and we avoid collisions _forever_!
	   If you need to select new classids for firewall rules or traffic control rules, keep
	   the major number "b054" for bosh stuff, unless there's a good reason to not.

	   The net_cls.classid structure is described in more detail here:
	   https://www.kernel.org/doc/Documentation/cgroup-v1/net_cls.txt
	*/
	natsIsolationClassID uint32 = 2958295042
)

// SetupNatsFirewall will setup the outgoing cgroup based rule that prevents everything except the agent to open connections to the nats api
func SetupNatsFirewall(mbus string) error {
	// return early if
	// we get a https url for mbus. case for create-env
	// we get an empty string. case for http_metadata_service (responsible to extract the agent-settings.json from the metadata endpoint)
	// we find that v1cgroups are not mounted (warden stemcells)
	if mbus == "" || strings.HasPrefix(mbus, "https://") {
		return nil
	}

	mbusURL, err := gonetURL.Parse(mbus)
	if err != nil || mbusURL.Hostname() == "" {
		return bosherr.WrapError(err, "Error parsing MbusURL")
	}

	host, port, err := net.SplitHostPort(mbusURL.Host)
	if err != nil {
		return bosherr.WrapError(err, "Error getting Port")
	}

	// Run the lookup for Host as it could be potentially a Hostname | IPv4 | IPv6
	// the return for LookupIP will be a list of IP Addr and in case of the Input being an IP Addr,
	// it will only contain one element with the Input IP
	addr_array, err := net.LookupIP(host)
	if err != nil {
		return bosherr.WrapError(err, fmt.Sprintf("Error resolving mbus host: %v", host))
	}

	if cgroups.Mode() == cgroups.Unified {
		return SetupNFTables(host, port)
	} else {
		return SetupIptables(host, port, addr_array)
	}
}

func SetupNFTables(host, port string) error {
	// NOBLE_TODO: check if warden does not hit this cgroup v2 code path
	conn := &nftables.Conn{}

	// Create or get the table
	table := &nftables.Table{
		Family: nftables.TableFamilyINet,
		Name:   "filter",
	}
	conn.AddTable(table)

	// Create the nats_postrouting chain
	// TODO: not sure if we still need a postrouting chain
	priority := nftables.ChainPriority(0)
	policy := nftables.ChainPolicyAccept
	postroutingChain := &nftables.Chain{
		Name:     "nats_postrouting",
		Table:    table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookPostrouting,
		Priority: &priority,
		Policy:   &policy,
	}

	conn.AddChain(postroutingChain)

	// Create the nats_output chain
	outputChain := &nftables.Chain{
		Name:     "nats_output",
		Table:    table,
		Hooknum:  nftables.ChainHookOutput,
		Priority: nftables.ChainPriorityFilter,
		Type:     nftables.ChainTypeFilter,
		Policy:   &policy,
	}
	conn.AddChain(outputChain)

	// Flush the chain
	conn.FlushChain(outputChain)

	// Function to convert IP to bytes
	ipToBytes := func(ipStr string) []byte {
		ip := net.ParseIP(ipStr).To4() //TODO: what if ip ipv6
		if ip == nil {
			return nil // TODO: handle log error case
		}
		return ip
	}

	// Function to convert port to bytes
	portToBytes := func(port string) []byte {
		// Convert port from string to int
		portInt, err := strconv.ParseInt(port, 10, 16)
		if err != nil {
			// return error if conversion fails
			return nil // TODO: handle log error case
		}

		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(portInt))
		return b
	}

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connd, err := dbus.NewWithContext(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to systemd: %v", err)
	}
	defer connd.Close()

	unitName := "bosh-agent.service"
	prop, err := connd.GetUnitTypePropertyContext(ctx, unitName, "Service", "ControlGroupId")
	if err != nil {
		log.Fatalf("Failed to get property: %v", err)
	}
	// Assuming prop.Value is of type dbus.Variant and contains a string
	unitControlGroupId, ok := prop.Value.Value().(uint64)
	if !ok {
		log.Fatalf("Expected unit64 value for ControlGroupId, got %T", prop.Value.Value())
	}

	// TODO: handle ipv6 case there is a funcation 'iPv46' in the nftables package. if we are going to support ipv6 communication from director to agent

	// Define the rule expressions
	rules := []struct {
		chain *nftables.Chain
		exprs []expr.Any
	}{
		{ // Rule 1: cgroup match
			// the folowing rule is created with chatgpt from the following nft command
			// `nft add rule inet filter nats_output socket cgroupv2 level 2 "system.slice/bosh-agent.service" ip daddr $host tcp dport $port log prefix "\"Matched cgroup bosh-agent nats rule: \"" accept`
			chain: outputChain,
			exprs: []expr.Any{
				&expr.Socket{Key: expr.SocketKeyCgroupv2, Level: 2, Register: 1},
				&expr.Cmp{Register: 1, Op: expr.CmpOpEq, Data: binaryutil.NativeEndian.PutUint64(unitControlGroupId)},
				&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{2}},
				&expr.Payload{OperationType: expr.PayloadLoad, Base: expr.PayloadBaseNetworkHeader, Offset: 16, Len: 4, DestRegister: 2},
				&expr.Cmp{Register: 2, Op: expr.CmpOpEq, Data: ipToBytes(host)},
				&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
				&expr.Cmp{Op: 0, Register: 1, Data: []byte{6}},
				&expr.Payload{OperationType: expr.PayloadLoad, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2, DestRegister: 3},
				&expr.Cmp{Register: 3, Op: expr.CmpOpEq, Data: portToBytes(port)},
				&expr.Log{Level: 4, Key: 36, Data: []byte("Matched cgroup bosh-agent nats rule: ")},
				&expr.Verdict{Kind: expr.VerdictAccept},
			},
		},
		{ // Rule 2: skuid match
			// `nft add rule inet filter nats_output skuid 0 ip daddr $host tcp dport $port log prefix "\"Matched skuid director nats rule: \"" accept`
			chain: outputChain,
			exprs: []expr.Any{
				&expr.Meta{Key: expr.MetaKeySKUID, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{0, 0, 0, 0}},
				&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{2}},
				&expr.Payload{OperationType: expr.PayloadLoad, Base: expr.PayloadBaseNetworkHeader, Offset: 16, Len: 4, DestRegister: 2},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 2, Data: ipToBytes(host)},
				&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
				&expr.Cmp{Op: 0, Register: 1, Data: []byte{6}},
				&expr.Payload{OperationType: expr.PayloadLoad, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2, DestRegister: 3},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 3, Data: portToBytes(port)},
				&expr.Log{Level: 4, Key: 36, Data: []byte("Matched skuid director nats rule: ")},
				&expr.Verdict{Kind: expr.VerdictAccept},
			},
		},
		{ // Rule 3: generic IP and port match
			// `nft add rule inet filter nats_output ip daddr $host tcp dport $port log prefix "\"dropped nats rule: \"" drop`
			chain: outputChain,
			exprs: []expr.Any{
				&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{2}},
				&expr.Payload{OperationType: expr.PayloadLoad, Base: expr.PayloadBaseNetworkHeader, Offset: 16, Len: 4, DestRegister: 1},
				&expr.Cmp{Register: 1, Op: expr.CmpOpEq, Data: ipToBytes(host)},
				&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
				&expr.Cmp{Op: 0, Register: 1, Data: []byte{6}},
				&expr.Payload{OperationType: expr.PayloadLoad, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2, DestRegister: 2},
				&expr.Cmp{Register: 2, Op: expr.CmpOpEq, Data: portToBytes(port)},
				&expr.Log{Level: 4, Key: 36, Data: []byte("dropped nats rule: ")},
				&expr.Verdict{Kind: expr.VerdictDrop},
			},
		},
	}

	// Add the new rules
	for _, r := range rules {
		// Add the new rule
		rule := &nftables.Rule{
			Table: table,
			Chain: r.chain,
			Exprs: r.exprs,
		}
		conn.AddRule(rule)
	}

	// Apply the changes
	if err := conn.Flush(); err != nil {
		return bosherr.WrapError(err, "Failed to apply nftables changes")
	}

	return nil
}

func SetupIptables(host, port string, addr_array []net.IP) error {
	_, err := cgroups.V1()
	if err != nil {
		if errors.Is(err, cgroups.ErrMountPointNotExist) {
			return nil // v1cgroups are not mounted (warden stemcells)
		}
		return bosherr.WrapError(err, "Error retrieving cgroups mount point")
	}

	ipt, err := iptables.New()
	if err != nil {
		return bosherr.WrapError(err, "Creating Iptables Error")
	}
	// Even on a V6 VM, Monit will listen to only V4 loopback
	// First create Monit V4 rules for natsIsolationClassID
	exists, err := ipt.Exists("mangle", "POSTROUTING",
		"-d", "127.0.0.1",
		"-p", "tcp",
		"--dport", "2822",
		"-m", "cgroup",
		"--cgroup", fmt.Sprintf("%v", natsIsolationClassID),
		"-j", "ACCEPT",
	)
	if err != nil {
		return bosherr.WrapError(err, "Iptables Error checking for monit rule")
	}
	if !exists {
		err = ipt.Insert("mangle", "POSTROUTING", 1,
			"-d", "127.0.0.1",
			"-p", "tcp",
			"--dport", "2822",
			"-m", "cgroup",
			"--cgroup", fmt.Sprintf("%v", natsIsolationClassID),
			"-j", "ACCEPT",
		)
		if err != nil {
			return bosherr.WrapError(err, "Iptables Error inserting for monit rule")
		}
	}

	// For nats iptables rules we default to V4 unless below dns resolution gives us a V6 target
	ipVersion := iptables.ProtocolIPv4
	// Check if we're dealing with a V4 Target
	if addr_array[0].To4() == nil {
		ipVersion = iptables.ProtocolIPv6
	}
	ipt, err = iptables.NewWithProtocol(ipVersion)
	if err != nil {
		return bosherr.WrapError(err, "Creating Iptables Error")
	}

	err = ipt.AppendUnique("mangle", "POSTROUTING",
		"-d", host,
		"-p", "tcp",
		"--dport", port,
		"-m", "cgroup",
		"--cgroup", fmt.Sprintf("%v", natsIsolationClassID),
		"-j", "ACCEPT",
	)
	if err != nil {
		return bosherr.WrapError(err, "Iptables Error inserting for agent ACCEPT rule")
	}
	err = ipt.AppendUnique("mangle", "POSTROUTING",
		"-d", host,
		"-p", "tcp",
		"--dport", port,
		"-j", "DROP",
	)
	if err != nil {
		return bosherr.WrapError(err, "Iptables Error inserting for non-agent DROP rule")
	}

	var isolationClassID = natsIsolationClassID
	natsAPICgroup, err := cgroups.New(cgroups.SingleSubsystem(cgroups.V1, cgroups.NetCLS), cgroups.StaticPath("/nats-api-access"), &specs.LinuxResources{
		Network: &specs.LinuxNetwork{
			ClassID: &isolationClassID,
		},
	})
	if err != nil {
		return bosherr.WrapError(err, "Error setting up cgroups for nats api access")
	}

	err = natsAPICgroup.AddProc(uint64(os.Getpid()), cgroups.NetCLS)
	return err
}
