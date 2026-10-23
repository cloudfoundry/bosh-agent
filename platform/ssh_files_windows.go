//go:build windows

package platform

import "fmt"

// Linux platform construction is not supported on Windows.
func setupSSHFiles(home, username string, keys []string) error {
	return fmt.Errorf("linux SSH setup is unavailable on Windows")
}
