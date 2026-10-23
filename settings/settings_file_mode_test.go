package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

func TestSettingsFileModesBeforeWriting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not available on Windows")
	}
	dir := t.TempDir()
	service := &settingsService{fs: system.NewOsFileSystem(boshlog.NewLogger(boshlog.LevelNone))}
	for _, existing := range []bool{false, true} {
		file := filepath.Join(dir, "settings.json")
		if existing {
			if err := os.Chmod(file, 0644); err != nil {
				t.Fatal(err)
			}
		}
		if err := service.writeSettingsFile(file, []byte("private-value")); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("settings mode %o", info.Mode().Perm())
		}
	}
}
