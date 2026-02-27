package firewall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

const cgroupLogTag = "cgroup"

// getCurrentCgroupPath reads /proc/self/cgroup and extracts the cgroupv2 path.
// Returns path WITHOUT leading slash (e.g., "system.slice/runc-bpm-galera-agent.scope")
// to match the format used by the nft CLI.
func getCurrentCgroupPath(logger boshlog.Logger) (string, error) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", fmt.Errorf("reading /proc/self/cgroup: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	logger.Debug(cgroupLogTag, "/proc/self/cgroup contents: %v", lines)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "0::") {
			path := strings.TrimPrefix(line, "0::")
			path = strings.TrimPrefix(path, "/")
			logger.Info(cgroupLogTag, "Detected cgroupv2 path: %s", path)
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
func isCgroupAccessible(logger boshlog.Logger, cgroupPath string) bool {
	fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)
	if _, err := os.Stat(fullPath); err != nil {
		logger.Info(cgroupLogTag, "Cgroup path doesn't exist: %s", fullPath)
		return false
	}

	controllers, err := os.ReadFile("/sys/fs/cgroup/cgroup.controllers")
	if err != nil {
		logger.Info(cgroupLogTag, "Cannot read cgroup.controllers: %v", err)
		return false
	}

	if len(strings.TrimSpace(string(controllers))) == 0 {
		logger.Info(cgroupLogTag, "Hybrid cgroup system detected (no controllers in cgroupv2)")
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
