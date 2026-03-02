package firewall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"golang.org/x/sys/unix"
)

const cgroupLogTag = "cgroup"

// getCurrentCgroupPath reads /proc/self/cgroup and determines the effective
// cgroup v2 path.
//
// Returns path WITHOUT leading slash to match the format used by the nft CLI.
// (e.g. "system.slice/runc-bpm-galera-agent.scope")
//
// On hybrid systems (e.g., Ubuntu Jammy), the path is automatically prefixed
// with "unified/" to align with the hybrid mount point.
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
			path, err := cgroupv2Path(path)
			if err != nil {
				return "", fmt.Errorf("determining cgroup v2 path: %w", err)
			}
			logger.Info(cgroupLogTag, "Detected cgroupv2 path: %s", path)
			return path, nil
		}
	}

	return "", fmt.Errorf("cgroupv2 path not found in /proc/self/cgroup")
}

// cgroupv2Path canonicalizes a path based on the detected cgroup hierarchy.
//
// On unified systems the path is returned unchanged
// on hybrid systems it is prefixed with "unified/" to match the cgroupv2
// mount at /sys/fs/cgroup/unified.
//
// Returns an error if the cgroup mode cannot be determined.
func cgroupv2Path(path string) (string, error) {
	switch detectCgroupMode() {
	case unifiedMode:
		return path, nil
	case hybridMode:
		return filepath.Join("unified", path), nil
	default:
		return "", fmt.Errorf("unknown cgroup mode")
	}
}

type cgroupMode int

const (
	unifiedMode cgroupMode = iota // Pure v2
	hybridMode                    // v1 with v2 at /unified
	unknownMode
)

// detectCgroupMode determines the system's cgroup hierarchy mode.
//
// Returns `unifiedMode`, if /sys/fs/cgroup is a cgroup2 filesystem.
// Returns `hybridMode`, if /sys/fs/cgroup/unified is a cgroup2 filesystem.
// Returns `unknownMode`, if cgroup2 was otherwise not detected.
func detectCgroupMode() cgroupMode {
	var st unix.Statfs_t

	if err := unix.Statfs("/sys/fs/cgroup", &st); err == nil && st.Type == unix.CGROUP2_SUPER_MAGIC {
		return unifiedMode
	}

	if err := unix.Statfs("/sys/fs/cgroup/unified", &st); err == nil && st.Type == unix.CGROUP2_SUPER_MAGIC {
		return hybridMode
	}

	return unknownMode
}

// getCgroupInodeID returns the inode ID for the cgroup path.
// The nftables kernel expects an 8-byte cgroup inode ID for 'socket cgroupv2'
// matching, NOT the path string. The nft CLI translates paths to inode IDs
// automatically, but the Go library requires manual lookup.
func getCgroupInodeID(cgroupPath string) (uint64, error) {
	fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)

	var stat unix.Stat_t
	if err := unix.Stat(fullPath, &stat); err != nil {
		return 0, fmt.Errorf("stat %s: %w", fullPath, err)
	}

	return stat.Ino, nil
}
