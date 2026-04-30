//go:build !windows

package net

// SetupNatsFirewall is a no-op on non-Windows platforms.
// On Linux, the nftables-based firewall in platform/firewall/ is used instead.
// On Windows, a Windows Firewall rule is set up.
func SetupNatsFirewall(mbus string) error {
	return nil
}
