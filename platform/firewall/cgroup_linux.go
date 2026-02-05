//go:build linux

package firewall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

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

// GetCgroupID returns the cgroup inode ID for the given cgroup path.
// This is used for nftables "socket cgroupv2" matching, which compares
// against the cgroup inode ID (not the path string).
//
// The cgroup path should be relative to /sys/fs/cgroup, e.g.:
// "/system.slice/bosh-agent.service" -> /sys/fs/cgroup/system.slice/bosh-agent.service
func GetCgroupID(cgroupPath string) (uint64, error) {
	// Construct the full path in the cgroup filesystem
	// The cgroup path from /proc/<pid>/cgroup is relative to the cgroup root
	fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)

	var stat syscall.Stat_t
	if err := syscall.Stat(fullPath, &stat); err != nil {
		return 0, fmt.Errorf("stat %s: %w", fullPath, err)
	}

	return stat.Ino, nil
}

// IsRunningUnderSystemd checks if the current process is managed by systemd.
// Returns true if the process is in a systemd-managed cgroup (*.service, *.scope, or *.slice).
//
// When running under systemd:
// - The agent will be in a cgroup like /system.slice/bosh-agent.service
// - Other processes will be in different cgroups (e.g., /user.slice/...)
// - Cgroup-based firewall isolation will work correctly
//
// When NOT running under systemd (e.g., in a container with runsv/runsvdir):
// - All processes share the same cgroup (typically "/" or the container's cgroup)
// - Cgroup-based firewall isolation cannot distinguish agent from other processes
// - The firewall rules are still created but blocking won't be effective
func IsRunningUnderSystemd() bool {
	version, err := DetectCgroupVersion()
	if err != nil {
		return false
	}

	cgroup, err := GetProcessCgroup(os.Getpid(), version)
	if err != nil {
		return false
	}

	return isSystemdCgroupPath(cgroup.Path)
}

// isSystemdCgroupPath returns true if the cgroup path indicates systemd management.
func isSystemdCgroupPath(path string) bool {
	return strings.Contains(path, ".service") ||
		strings.Contains(path, ".scope") ||
		strings.Contains(path, ".slice")
}
