package disk_test

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudfoundry/bosh-agent/v2/platform/disk"
	"github.com/cloudfoundry/bosh-agent/v2/platform/disk/diskfakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

func TestDiskFileContentExcludedFromLogs(t *testing.T) {
	var output bytes.Buffer
	logger := boshlog.New(boshlog.LevelDebug, log.New(&output, "", 0))
	fs := system.NewOsFileSystem(logger)
	mounter := &diskfakes.FakeMounter{}
	contents := `{"mbus":"nats://user:private-password@host"}`
	mounter.MountStub = func(_, target string, _ ...string) error {
		return os.WriteFile(filepath.Join(target, "settings.json"), []byte(contents), 0600)
	}
	util := disk.NewUtil(nil, mounter, fs, logger)
	got, err := util.GetFilesContents(t.TempDir(), []string{"settings.json"})
	if err != nil || len(got) != 1 || string(got[0]) != contents {
		t.Fatalf("disk file contents: %v %v", got, err)
	}
	if strings.Contains(output.String(), "private-password") {
		t.Fatal("disk settings content appeared in logs")
	}
}
