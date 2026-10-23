package cert

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	boshdir "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system/fakes"
)

func TestWindowsCertificateInputUsesStdin(t *testing.T) {
	runner := fakes.NewFakeCmdRunner()
	manager := NewWindowsCertManager(fakes.NewFakeFileSystem(), runner, boshdir.NewProvider("directory'with-quote"), logger.NewLogger(logger.LevelNone))
	if err := manager.UpdateCertificates(""); err != nil {
		t.Fatal(err)
	}
	if len(runner.RunCommandsWithInput) != 1 || len(runner.RunCommands) != 0 {
		t.Fatal("certificate update escaped the single checked script")
	}
	args := runner.RunCommandsWithInput[0]
	var config struct {
		BackupDirectory, LegacyPath string
		Certificates                []string
	}
	if err := json.Unmarshal([]byte(args[0]), &config); err != nil {
		t.Fatal(err)
	}
	if config.BackupDirectory != filepath.Join("directory'with-quote", "bosh", "root-cert-backup") {
		t.Fatal(config)
	}
	if strings.Contains(strings.Join(args[1:], " "), "directory'with-quote") {
		t.Fatal("path interpolated into executable script")
	}
	if !strings.Contains(args[len(args)-1], "exit 1") {
		t.Fatal("PowerShell errors must fail closed")
	}
	failure := errors.New("baseline rejected")
	runner.AddCmdResult(strings.Join(args, " "), fakes.FakeCmdResult{Error: failure})
	if err := manager.UpdateCertificates(""); !errors.Is(err, failure) {
		t.Fatal("backup validation failure did not stop update", err)
	}
}

func TestWindowsCertificateScriptSafety(t *testing.T) {
	program := os.Getenv("BOSH_TEST_POWERSHELL")
	if program == "" {
		var err error
		program, err = exec.LookPath("pwsh")
		if err != nil {
			t.Skip("PowerShell runtime unavailable; mock script tests require pwsh")
		}
	}
	output, err := exec.Command(program, "-NoProfile", "-NonInteractive", "-File", "windows_cert_store_test.ps1").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	t.Log(string(output))
}

func TestWindowsCertificateProcessRejectsLegacy(t *testing.T) {
	program := os.Getenv("BOSH_TEST_POWERSHELL")
	if program == "" {
		var err error
		program, err = exec.LookPath("pwsh")
		if err != nil {
			t.Skip("PowerShell runtime unavailable")
		}
	}
	legacy := filepath.Join(t.TempDir(), "rootCertBackup.sst")
	if err := os.WriteFile(legacy, []byte("untrusted"), 0600); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "private")
	config, err := json.Marshal(map[string]interface{}{"LegacyPath": legacy, "BackupDirectory": backup, "Certificates": []string{}})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(program, "-NoProfile", "-NonInteractive", "-Command", windowsCertStoreScript+windowsCertStoreEntry)
	command.Stdin = strings.NewReader(string(config))
	output, err := command.CombinedOutput()
	var failure *exec.ExitError
	if !errors.As(err, &failure) || failure.ExitCode() != 1 || !strings.Contains(string(output), "Legacy root certificate backup is untrusted") {
		t.Fatalf("must reject legacy baseline before store mutation: %v: %s", err, output)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatal("legacy rejection did not stop preparation", err)
	}
}
