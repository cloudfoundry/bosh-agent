package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"github.com/cloudfoundry/bosh-agent/v2/app"
	boshinf "github.com/cloudfoundry/bosh-agent/v2/infrastructure"
)

func TestDefaultAgentConfigRoundTrips(t *testing.T) {
	configJSON, err := json.Marshal(DefaultAgentConfig)
	if err != nil {
		t.Fatalf("marshalling DefaultAgentConfig: %s", err)
	}

	path := filepath.Join(t.TempDir(), "agent.json")
	if err := os.WriteFile(path, configJSON, 0644); err != nil {
		t.Fatalf("writing config: %s", err)
	}

	fs := boshsys.NewOsFileSystem(boshlog.NewLogger(boshlog.LevelNone))
	loaded, err := app.LoadConfigFromPath(fs, path)
	if err != nil {
		t.Fatalf("loading config from %s (marshaled: %s): %s", path, configJSON, err)
	}

	sources := loaded.Infrastructure.Settings.Sources
	if len(sources) != 1 {
		t.Fatalf("expected exactly one settings source after round-trip, got %d (marshaled: %s)", len(sources), configJSON)
	}

	fileSource, ok := sources[0].(boshinf.FileSourceOptions)
	if !ok {
		t.Fatalf("expected a FileSourceOptions after round-trip, got %T", sources[0])
	}
	if fileSource.SettingsPath != "/var/vcap/settings.json" {
		t.Fatalf("expected SettingsPath /var/vcap/settings.json, got %q", fileSource.SettingsPath)
	}
}
