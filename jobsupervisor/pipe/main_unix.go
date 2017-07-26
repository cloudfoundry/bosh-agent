// +build !windows

// Pipe runs a process and redirects its stdout/stderr to its own stdout/stderr,
// the Windows Event Log and optionally syslog.
package main

// This files exists only to prevent 'no buildable Go source files' errors

func main() {
	panic("Pipe is only supported on Windows")
}
