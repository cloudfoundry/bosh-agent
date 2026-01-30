//go:build !linux

package firewall

import "fmt"

// DetectCgroupVersion is not supported on non-Linux platforms
func DetectCgroupVersion() (CgroupVersion, error) {
	return CgroupV1, fmt.Errorf("cgroup detection not supported on this platform")
}

// IsCgroupV2SocketMatchFunctional is not supported on non-Linux platforms
func IsCgroupV2SocketMatchFunctional() bool {
	return false
}

// GetProcessCgroup is not supported on non-Linux platforms
func GetProcessCgroup(pid int, version CgroupVersion) (ProcessCgroup, error) {
	return ProcessCgroup{}, fmt.Errorf("cgroup not supported on this platform")
}

// ReadOperatingSystem is not supported on non-Linux platforms
func ReadOperatingSystem() (string, error) {
	return "", fmt.Errorf("operating system detection not supported on this platform")
}
