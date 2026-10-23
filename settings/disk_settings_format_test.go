package settings_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudfoundry/bosh-agent/v2/settings"
)

func TestDiskSettingsDisplayAndSerialization(t *testing.T) {
	disk := settings.DiskSettings{ID: "disk-id", ISCSISettings: settings.ISCSISettings{Target: "target-host", Password: "private-chap-value"}}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		display := fmt.Sprintf(format, disk)
		if strings.Contains(display, "private-chap-value") {
			t.Fatalf("format %s includes authentication value", format)
		}
		if !strings.Contains(display, "disk-id") || !strings.Contains(display, "target-host") {
			t.Fatalf("format %s lost disk identity", format)
		}
	}
	encoded, err := json.Marshal(disk)
	if err != nil || !strings.Contains(string(encoded), "private-chap-value") {
		t.Fatal("disk authentication serialization changed")
	}
}
