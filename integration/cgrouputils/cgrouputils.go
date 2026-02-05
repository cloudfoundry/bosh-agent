// Package cgrouputils provides utilities for diagnosing cgroup configuration
// in nested container environments. This is used by integration tests to
// understand and debug cgroup-related issues with the BOSH agent's nftables firewall.
package cgrouputils

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry/bosh-agent/v2/integration/installerdriver"
	"github.com/onsi/ginkgo/v2"
)

// CgroupDiagnostics holds diagnostic information about the cgroup environment.
type CgroupDiagnostics struct {
	// ProcessCgroupPath is the cgroup path from /proc/self/cgroup
	ProcessCgroupPath string

	// CgroupHierarchy lists the contents of /sys/fs/cgroup (top level)
	CgroupHierarchy []string

	// NestingDepth is the number of path components in the cgroup path
	// (e.g., "/system.slice/garden.service/container" has depth 3)
	NestingDepth int

	// CgroupMounted indicates whether /sys/fs/cgroup is accessible
	CgroupMounted bool

	// CgroupV2 indicates whether cgroup v2 unified hierarchy is in use
	CgroupV2 bool

	// RawProcCgroup contains the raw contents of /proc/self/cgroup
	RawProcCgroup string

	// Error contains any error encountered during collection
	Error error
}

// CollectDiagnostics gathers cgroup diagnostic information from the target environment.
func CollectDiagnostics(driver installerdriver.Driver) *CgroupDiagnostics {
	diag := &CgroupDiagnostics{}

	// Check if cgroup is mounted
	diag.CgroupMounted = IsCgroupMounted(driver)

	// Get process cgroup path
	cgroupPath, err := GetProcessCgroup(driver)
	if err != nil {
		diag.Error = err
	} else {
		diag.ProcessCgroupPath = cgroupPath
		diag.NestingDepth = GetNestingDepth(cgroupPath)
	}

	// Get raw /proc/self/cgroup content
	stdout, _, exitCode, err := driver.RunCommand("cat", "/proc/self/cgroup")
	if err == nil && exitCode == 0 {
		diag.RawProcCgroup = strings.TrimSpace(stdout)
	}

	// Check if cgroup v2 unified hierarchy
	diag.CgroupV2 = IsCgroupV2(driver)

	// Get cgroup hierarchy contents if mounted
	if diag.CgroupMounted {
		stdout, _, exitCode, err := driver.RunCommand("ls", "-1", "/sys/fs/cgroup")
		if err == nil && exitCode == 0 {
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			for _, line := range lines {
				if line != "" {
					diag.CgroupHierarchy = append(diag.CgroupHierarchy, line)
				}
			}
		}
	}

	return diag
}

// GetProcessCgroup returns the cgroup path for the current process.
// For cgroup v2, this is the path from the "0::" line in /proc/self/cgroup.
// For cgroup v1, this returns the path from the first controller.
func GetProcessCgroup(driver installerdriver.Driver) (string, error) {
	stdout, stderr, exitCode, err := driver.RunCommand("cat", "/proc/self/cgroup")
	if err != nil {
		return "", fmt.Errorf("failed to read /proc/self/cgroup: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("failed to read /proc/self/cgroup: exit %d, stderr: %s", exitCode, stderr)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}

		// For cgroup v2, look for "0::" line
		if parts[0] == "0" && parts[1] == "" {
			return parts[2], nil
		}
	}

	// Fallback: return the first controller's path (cgroup v1)
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 {
			return parts[2], nil
		}
	}

	return "", fmt.Errorf("could not parse cgroup path from: %s", stdout)
}

// IsCgroupMounted checks if /sys/fs/cgroup is accessible.
func IsCgroupMounted(driver installerdriver.Driver) bool {
	_, _, exitCode, err := driver.RunCommand("test", "-d", "/sys/fs/cgroup")
	return err == nil && exitCode == 0
}

// IsCgroupV2 checks if cgroup v2 unified hierarchy is in use.
// This is detected by the presence of "cgroup.controllers" in /sys/fs/cgroup.
func IsCgroupV2(driver installerdriver.Driver) bool {
	_, _, exitCode, err := driver.RunCommand("test", "-f", "/sys/fs/cgroup/cgroup.controllers")
	return err == nil && exitCode == 0
}

// GetNestingDepth returns the number of path components in a cgroup path.
// Empty paths and "/" return 0.
func GetNestingDepth(cgroupPath string) int {
	if cgroupPath == "" || cgroupPath == "/" {
		return 0
	}

	// Remove leading slash and count components
	path := strings.TrimPrefix(cgroupPath, "/")
	if path == "" {
		return 0
	}

	return len(strings.Split(path, "/"))
}

// LogDiagnostics logs the cgroup diagnostics using GinkgoWriter.
func LogDiagnostics(diag *CgroupDiagnostics) {
	ginkgo.GinkgoWriter.Println("=== Cgroup Diagnostics ===")
	ginkgo.GinkgoWriter.Printf("  Cgroup Mounted: %v\n", diag.CgroupMounted)
	ginkgo.GinkgoWriter.Printf("  Cgroup V2: %v\n", diag.CgroupV2)
	ginkgo.GinkgoWriter.Printf("  Process Cgroup Path: %s\n", diag.ProcessCgroupPath)
	ginkgo.GinkgoWriter.Printf("  Nesting Depth: %d\n", diag.NestingDepth)

	if diag.RawProcCgroup != "" {
		ginkgo.GinkgoWriter.Println("  Raw /proc/self/cgroup:")
		for _, line := range strings.Split(diag.RawProcCgroup, "\n") {
			ginkgo.GinkgoWriter.Printf("    %s\n", line)
		}
	}

	if len(diag.CgroupHierarchy) > 0 {
		ginkgo.GinkgoWriter.Println("  Cgroup Hierarchy (/sys/fs/cgroup):")
		for _, entry := range diag.CgroupHierarchy {
			ginkgo.GinkgoWriter.Printf("    %s\n", entry)
		}
	}

	if diag.Error != nil {
		ginkgo.GinkgoWriter.Printf("  Error: %v\n", diag.Error)
	}

	ginkgo.GinkgoWriter.Println("==========================")
}

// LogDiagnosticsf logs the cgroup diagnostics with a custom prefix format.
func LogDiagnosticsf(prefix string, diag *CgroupDiagnostics) {
	ginkgo.GinkgoWriter.Printf("=== %s Cgroup Diagnostics ===\n", prefix)
	ginkgo.GinkgoWriter.Printf("  Cgroup Mounted: %v\n", diag.CgroupMounted)
	ginkgo.GinkgoWriter.Printf("  Cgroup V2: %v\n", diag.CgroupV2)
	ginkgo.GinkgoWriter.Printf("  Process Cgroup Path: %s\n", diag.ProcessCgroupPath)
	ginkgo.GinkgoWriter.Printf("  Nesting Depth: %d\n", diag.NestingDepth)

	if diag.RawProcCgroup != "" {
		ginkgo.GinkgoWriter.Println("  Raw /proc/self/cgroup:")
		for _, line := range strings.Split(diag.RawProcCgroup, "\n") {
			ginkgo.GinkgoWriter.Printf("    %s\n", line)
		}
	}

	if len(diag.CgroupHierarchy) > 0 {
		ginkgo.GinkgoWriter.Println("  Cgroup Hierarchy (/sys/fs/cgroup):")
		for _, entry := range diag.CgroupHierarchy {
			ginkgo.GinkgoWriter.Printf("    %s\n", entry)
		}
	}

	if diag.Error != nil {
		ginkgo.GinkgoWriter.Printf("  Error: %v\n", diag.Error)
	}

	ginkgo.GinkgoWriter.Println(strings.Repeat("=", len(prefix)+29))
}

// GetCgroupLevel returns the effective cgroup level for socket cgroup matching.
// This is what the kernel would evaluate for "socket cgroupv2 level N" in nftables.
//
// The level calculation:
//   - Level 1: root cgroup ("/")
//   - Level 2: first component (e.g., "/system.slice")
//   - Level 3: second component (e.g., "/system.slice/garden.service")
//   - etc.
//
// For nested containers, the kernel evaluates against the GLOBAL cgroup hierarchy,
// not the container's namespaced view. This is the root cause of the firewall issue.
func GetCgroupLevel(cgroupPath string) int {
	if cgroupPath == "" || cgroupPath == "/" {
		return 1
	}
	// Level = depth + 1 (root is level 1)
	return GetNestingDepth(cgroupPath) + 1
}

// IsSystemdAvailable checks if systemd is managing processes in this environment.
// Returns true if processes are running under a systemd-managed cgroup hierarchy.
//
// Systemd places processes in cgroups with paths like:
//   - /system.slice/bosh-agent.service (for system services)
//   - /user.slice/user-1000.slice/session-1.scope (for user sessions)
//   - /init.scope (for PID 1 itself)
//
// This function checks if the current process is in such a cgroup.
func IsSystemdAvailable(driver installerdriver.Driver) bool {
	cgroupPath, err := GetProcessCgroup(driver)
	if err != nil {
		return false
	}
	return isSystemdCgroupPath(cgroupPath)
}

// isSystemdCgroupPath returns true if the cgroup path indicates systemd management.
func isSystemdCgroupPath(cgroupPath string) bool {
	return strings.Contains(cgroupPath, ".service") ||
		strings.Contains(cgroupPath, ".scope") ||
		strings.Contains(cgroupPath, ".slice")
}
