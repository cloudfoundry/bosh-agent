package cert

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"path/filepath"

	"github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshdir "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

type windowsCertManager struct {
	runner          boshsys.CmdRunner
	backupDirectory string
	legacyPath      string
}

//go:embed windows_cert_store.ps1
var windowsCertStoreScript string

const windowsCertStoreEntry = `
try {
    $ErrorActionPreference = 'Stop'
    Invoke-RootCertificateUpdate (ConvertFrom-Json ([Console]::In.ReadToEnd()))
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
}
`

func NewWindowsCertManager(_ boshsys.FileSystem, runner boshsys.CmdRunner, dirProvider boshdir.Provider, _ logger.Logger) Manager {
	return &windowsCertManager{
		runner:          runner,
		backupDirectory: filepath.Join(dirProvider.BoshDir(), "root-cert-backup"),
		legacyPath:      filepath.Join(dirProvider.TmpDir(), "rootCertBackup.sst"),
	}
}

func (c *windowsCertManager) UpdateCertificates(rawCerts string) error {
	certificates := []string{}
	for _, certificate := range splitCerts(rawCerts) {
		block, _ := pem.Decode([]byte(certificate))
		if block == nil || block.Type != "CERTIFICATE" {
			return fmt.Errorf("invalid trusted certificate")
		}
		certificates = append(certificates, base64.StdEncoding.EncodeToString(block.Bytes))
	}
	input, err := json.Marshal(struct {
		BackupDirectory string
		LegacyPath      string
		Certificates    []string
	}{c.backupDirectory, c.legacyPath, certificates})
	if err != nil {
		return err
	}
	_, _, _, err = c.runner.RunCommandWithInput(string(input), "powershell", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", windowsCertStoreScript+windowsCertStoreEntry)
	return err
}
