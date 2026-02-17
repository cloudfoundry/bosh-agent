package monitaccess

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// getCurrentCgroupPath reads /proc/self/cgroup and extracts the cgroupv2 path.
// Returns path WITHOUT leading slash (e.g., "system.slice/runc-bpm-galera-agent.scope")
// to match the format used by the nft CLI.
func getCurrentCgroupPath() (string, error) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", fmt.Errorf("reading /proc/self/cgroup: %w", err)
	}

	// Find line starting with "0::" (cgroupv2)
	// Format: "0::/system.slice/runc-bpm-galera-agent.scope"
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "0::") {
			path := strings.TrimPrefix(line, "0::")
			// Strip leading slash to match Noble script format
			path = strings.TrimPrefix(path, "/")
			return path, nil
		}
	}

	return "", fmt.Errorf("cgroupv2 path not found in /proc/self/cgroup")
}

// isCgroupAccessible checks if the cgroup path is accessible and functional
// for nftables socket cgroupv2 matching.
//
// This returns false in these cases:
// - Cgroup path doesn't exist in /sys/fs/cgroup
// - Hybrid cgroup system (cgroupv2 mounted but no controllers delegated)
// - Nested containers where cgroup path is different from host view
func isCgroupAccessible(cgroupPath string) bool {
	// Check if cgroup path exists
	fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)
	if _, err := os.Stat(fullPath); err != nil {
		fmt.Printf("bosh-monit-access: Cgroup path doesn't exist: %s\n", fullPath)
		return false
	}

	// Check if this is a hybrid cgroup system (cgroupv2 mounted but no controllers)
	// On hybrid systems, /sys/fs/cgroup/cgroup.controllers exists but is empty
	controllers, err := os.ReadFile("/sys/fs/cgroup/cgroup.controllers")
	if err != nil {
		fmt.Printf("bosh-monit-access: Cannot read cgroup.controllers: %v\n", err)
		return false
	}

	if len(strings.TrimSpace(string(controllers))) == 0 {
		fmt.Println("bosh-monit-access: Hybrid cgroup system detected (no controllers in cgroupv2)")
		return false
	}

	return true
}

// getCgroupInodeID returns the inode ID for the cgroup path.
// The nftables kernel expects an 8-byte cgroup inode ID for 'socket cgroupv2'
// matching, NOT the path string. The nft CLI translates paths to inode IDs
// automatically, but the Go library requires manual lookup.
func getCgroupInodeID(cgroupPath string) (uint64, error) {
	fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)

	var stat syscall.Stat_t
	if err := syscall.Stat(fullPath, &stat); err != nil {
		return 0, fmt.Errorf("stat %s: %w", fullPath, err)
	}

	return stat.Ino, nil
}
