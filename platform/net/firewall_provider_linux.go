//go:build linux
// +build linux

package net

import (
	"errors"
	"fmt"
	"net"
	gonetURL "net/url"
	"os"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/containerd/cgroups" // NOTE: linux only; see: https://github.com/containerd/cgroups/issues/19
	"github.com/coreos/go-iptables/iptables"
	"github.com/opencontainers/runtime-spec/specs-go"
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
	// We have decided to remove the NATS firewall starting with Noble because we have
	// ephemeral NATS credentials implemented in the Bosh Director which is a better solution
	// to the problem. This allows us to remove all of this code after Jammy support ends
	if cgroups.Mode() == cgroups.Unified {
		return nil
	}

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

	return SetupIptables(host, port, addr_array)
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
