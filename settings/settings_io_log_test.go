package settings_test

import (
	"bytes"
	"log"
	"path/filepath"
	"strings"
	"testing"

	sourcefakes "github.com/cloudfoundry/bosh-agent/v2/infrastructure/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/settings"
	"github.com/cloudfoundry/bosh-agent/v2/settings/settingsfakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
)

func TestSettingsIOExcludesContentsFromLogs(t *testing.T) {
	var output bytes.Buffer
	logger := boshlog.New(boshlog.LevelDebug, log.New(&output, "", 0))
	fs := system.NewOsFileSystem(logger)
	dir := t.TempDir()
	platform := &settingsfakes.FakePlatformSettingsGetter{}
	platform.GetAgentSettingsPathReturns(filepath.Join(dir, "settings.json"))
	platform.GetUpdateSettingsPathReturns(filepath.Join(dir, "update_settings.json"))
	platform.GetPersistentDiskSettingsPathReturns(filepath.Join(dir, "persistent_disk_settings.json"))
	service := settings.NewService(fs, &sourcefakes.FakeSettingsSource{}, platform, logger)
	marker := "private-credential-value"
	update := settings.UpdateSettings{Blobstores: []settings.Blobstore{{Type: "s3", Options: map[string]interface{}{"secret_access_key": marker}}}}
	if err := service.SaveUpdateSettings(update); err != nil {
		t.Fatal(err)
	}
	if err := service.SavePersistentDiskSettings(settings.DiskSettings{ID: "disk", ISCSISettings: settings.ISCSISettings{Password: marker}}); err != nil {
		t.Fatal(err)
	}
	if err := service.LoadSettings(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetAllPersistentDiskSettings(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), marker) {
		t.Fatal("settings credentials were included in filesystem logs")
	}
	data, err := fs.ReadFileWithOpts(platform.GetUpdateSettingsPath(false), system.ReadOpts{Quiet: true})
	if err != nil || !strings.Contains(string(data), marker) {
		t.Fatalf("settings not preserved: %v", err)
	}
}
