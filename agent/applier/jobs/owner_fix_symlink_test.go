package jobs_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/jobs"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

type permissionFS struct {
	system.FileSystem
	chowned []string
}

func (fs *permissionFS) Chown(name, _ string) error {
	fs.chowned = append(fs.chowned, name)
	return nil
}
func TestPermissionFixLeavesSymlinkTargetsAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX permission bits to compare")
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "job")
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0750); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.WriteFile(outside, []byte("private-value"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(bin, "external")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	fs := &permissionFS{FileSystem: system.NewOsFileSystem(boshlog.NewLogger(boshlog.LevelNone))}
	if err := jobs.FixPermissions(fs, root, "root", "group"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("external target mode changed to %o", info.Mode().Perm())
	}
	for _, name := range fs.chowned {
		if name == link {
			t.Fatal("ownership operation followed archive symlink")
		}
	}
}
