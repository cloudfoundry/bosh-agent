package jobsupervisor

import "testing"

func TestMonitJobNamesCheckedBeforePaths(t *testing.T) {
	for _, name := range []string{"../outside", "..", `..\outside`, "/outside", ""} {
		if err := (monitJobSupervisor{}).AddJob(name, 0, "config"); err == nil {
			t.Fatalf("accepted job %q", name)
		}
	}
}
