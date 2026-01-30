//go:build linux

package firewall

import (
	"fmt"
	"os"
	"strings"

	cgroups "github.com/containerd/cgroups/v3"
)

// DetectCgroupVersion detects the cgroup version at runtime by checking
// whether the system is using unified (v2) or legacy (v1) cgroup hierarchy.
// This correctly handles:
// - Jammy VM on Jammy host: Detects cgroup v1
// - Jammy container on Noble host: Detects cgroup v2 (inherits from host!)
// - Noble anywhere: Detects cgroup v2
func DetectCgroupVersion() (CgroupVersion, error) {
	if cgroups.Mode() == cgroups.Unified {
		return CgroupV2, nil
	}
	return CgroupV1, nil
}

// IsCgroupV2SocketMatchFunctional returns true if nftables "socket cgroupv2"
// matching will work. This requires a pure cgroup v2 system with controllers
// enabled. On hybrid cgroup systems (cgroup v2 mounted but with no controllers
// enabled), the socket-to-cgroup association doesn't work for nftables matching.
//
// Hybrid cgroup is detected by checking if /sys/fs/cgroup/cgroup.controllers
// exists and is empty (no controllers delegated to cgroup v2).
func IsCgroupV2SocketMatchFunctional() bool {
	// First check if we're even on cgroup v2
	if cgroups.Mode() != cgroups.Unified {
		return false
	}

	// Check if cgroup v2 has controllers enabled
	// On hybrid systems, this file exists but is empty
	controllers, err := os.ReadFile("/sys/fs/cgroup/cgroup.controllers")
	if err != nil {
		// File doesn't exist or can't be read - assume not functional
		return false
	}

	// If empty, cgroup v2 is mounted but has no controllers - socket matching won't work
	return len(strings.TrimSpace(string(controllers))) > 0
}

// GetProcessCgroup gets the cgroup identity for a process by reading /proc/<pid>/cgroup
func GetProcessCgroup(pid int, version CgroupVersion) (ProcessCgroup, error) {
	cgroupFile := fmt.Sprintf("/proc/%d/cgroup", pid)
	data, err := os.ReadFile(cgroupFile)
	if err != nil {
		return ProcessCgroup{}, fmt.Errorf("reading %s: %w", cgroupFile, err)
	}

	if version == CgroupV2 {
		return parseCgroupV2(string(data))
	}
	return parseCgroupV1(string(data))
}

// parseCgroupV2 extracts the cgroup path from /proc/<pid>/cgroup for cgroup v2
// Format: "0::/system.slice/bosh-agent.service"
func parseCgroupV2(data string) (ProcessCgroup, error) {
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "0::") {
			path := strings.TrimPrefix(line, "0::")
			return ProcessCgroup{
				Version: CgroupV2,
				Path:    path,
			}, nil
		}
	}
	return ProcessCgroup{}, fmt.Errorf("cgroup v2 path not found in /proc/self/cgroup")
}

// parseCgroupV1 extracts the cgroup info from /proc/<pid>/cgroup for cgroup v1
// Format: "12:net_cls,net_prio:/system.slice/bosh-agent.service"
func parseCgroupV1(data string) (ProcessCgroup, error) {
	// Look for net_cls controller which is used for firewall matching
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "net_cls") {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 3 {
				return ProcessCgroup{
					Version: CgroupV1,
					Path:    parts[2],
					// ClassID will be set when the process is added to the cgroup
				}, nil
			}
		}
	}

	// Fallback: return empty path, will use classid-based matching
	return ProcessCgroup{
		Version: CgroupV1,
	}, nil
}

// ReadOperatingSystem reads the operating system name from the BOSH-managed file
func ReadOperatingSystem() (string, error) {
	data, err := os.ReadFile("/var/vcap/bosh/etc/operating_system")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
