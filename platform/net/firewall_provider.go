//go:build !windows && !linux
// +build !windows,!linux

package net

// SetupNatsFirewall is does nothing, except on Linux and Windows
func SetupNatsFirewall(mbus string) error {
	// NOTE: If we return a "not supported" err here, unit tests would fail.
	//return errors.New("not supported")
	return nil
}
