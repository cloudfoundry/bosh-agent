package utils

import "testing"

func TestIsPathComponent(t *testing.T) {
	for _, name := range []string{"job", "disk-1", "caching:None;disk_name:bosh-disk-data-1;resource_group_name:rg", "-dash"} {
		if !IsPathComponent(name) {
			t.Errorf("rejected %q", name)
		}
	}
	for _, name := range []string{"", ".", "..", "../outside", "/outside", "a/b", `..\outside`, "a\x00b"} {
		if IsPathComponent(name) {
			t.Errorf("accepted %q", name)
		}
	}
}
