package platform

import (
	"testing"

	"github.com/cloudfoundry/bosh-agent/v2/settings"
)

func TestDiskNamesCheckedBeforeFilesystemUse(t *testing.T) {
	platform := linux{}
	for _, name := range []string{"../outside", "..", ".", "", `..\outside`, "/outside"} {
		if err := platform.AssociateDisk(name, settings.DiskSettings{}); err == nil {
			t.Fatalf("accepted disk association %q", name)
		}
		if err := platform.SetupDynamicDisk(settings.DiskSettings{ID: name}); err == nil {
			t.Fatalf("accepted disk setup %q", name)
		}
		if err := platform.CleanupDynamicDisk(name); err == nil {
			t.Fatalf("accepted disk cleanup %q", name)
		}
	}
}
