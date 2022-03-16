//go:build !windows
// +build !windows

package net

import (
	"errors"
	"fmt"
	"net"
	gonetURL "net/url"
	"os"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/containerd/cgroups"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/coreos/go-iptables/iptables"
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
	_, err := cgroups.V1()
	if err != nil {
		if errors.Is(err, cgroups.ErrMountPointNotExist) {
			return nil // v1cgroups are not mounted (warden stemcells)
		} else {
			return bosherr.WrapError(err, "Error retrieving cgroups mount point")
		}
	}
	mbusURL, err := gonetURL.Parse(mbus)
	if err != nil || mbusURL.Hostname() == "" {
		return bosherr.WrapError(err, "Error parsing MbusURL")
	}

	host, port, err := net.SplitHostPort(mbusURL.Host)
	if err != nil {
		return bosherr.WrapError(err, "Error getting Port")
	}
	ipt, err := iptables.New()
	if err != nil {
		return bosherr.WrapError(err, "Iptables Error")
	}
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
