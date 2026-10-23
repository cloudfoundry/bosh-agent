package ini_test

import (
	"bytes"
	"testing"

	"github.com/cloudfoundry/bosh-agent/v2/ini"
)

func TestINIRejectsStructuralValuesBeforeWriting(t *testing.T) {
	for _, test := range []struct{ section, key, value string }{
		{"Network", "Gateway", "10.0.0.1\n[Match]\nName=*"},
		{"Network", "DNS", "10.0.0.1\r\n"},
		{"Network", "Name\nOther", "value"},
		{"Network]\n[Match", "Name", "value"},
		{"Network", "DNS", "10.0.0.1\x00"},
	} {
		file := ini.Empty()
		section := &ini.Section{Name: test.section}
		section.AddKey(test.key, test.value)
		file.AppendSection(section)
		var out bytes.Buffer
		n, err := file.WriteTo(&out)
		if err == nil || n != 0 || out.Len() != 0 {
			t.Fatalf("invalid INI input wrote output: %d %v %q", n, err, out.String())
		}
	}
}
func TestINIValidNetworkValue(t *testing.T) {
	file := ini.Empty()
	section := &ini.Section{Name: "Network"}
	section.AddKey("DNS", "10.0.0.1 2001:db8::1")
	file.AppendSection(section)
	var out bytes.Buffer
	if _, err := file.WriteTo(&out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "[Network]\nDNS=10.0.0.1 2001:db8::1\n\n" {
		t.Fatalf("unexpected config %q", out.String())
	}
}
