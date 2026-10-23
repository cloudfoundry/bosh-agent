package platform

import (
	"testing"

	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

func TestLogrotateSizeValue(t *testing.T) {
	for _, size := range []string{"50M\n}\n/etc/passwd {\npostrotate\ncmd\nendscript\n", "50M\r\n", "50M }", "", "50M\x00"} {
		platform := linux{}
		if err := platform.SetupLogrotate("vcap", "/var/vcap", size); err == nil {
			t.Fatalf("accepted invalid logrotate size %q", size)
		}
	}
	for _, size := range []string{"100", "100k", "50M", "1G"} {
		platform := linux{fs: fakesys.NewFakeFileSystem(), cmdRunner: fakesys.NewFakeCmdRunner()}
		if err := platform.SetupLogrotate("vcap", "/var/vcap", size); err != nil {
			t.Fatalf("valid size %q: %v", size, err)
		}
	}
}
