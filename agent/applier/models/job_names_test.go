package models

import "testing"

func TestJobDirectoryNamesCheckedBeforePaths(t *testing.T) {
	for _, name := range []string{"../outside", "..", `..\outside`, "/outside"} {
		if err := (Job{Name: name}).CreateDirectories(nil, nil); err == nil {
			t.Fatalf("accepted job %q", name)
		}
	}
}
