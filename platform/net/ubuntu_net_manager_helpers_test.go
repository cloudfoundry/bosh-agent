//go:build !windows

package net_test

import "fmt"

func netFile(iface string) string {
	return fmt.Sprintf("/etc/systemd/network/10-%s.network", iface)
}
