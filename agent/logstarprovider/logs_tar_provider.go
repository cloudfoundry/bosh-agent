package logstarprovider

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"

	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
)

type logsTarProvider struct {
	compressor  boshcmd.Compressor
	copier      boshcmd.Copier
	settingsDir boshdirs.Provider
}

func NewLogsTarProvider(
	compressor boshcmd.Compressor,
	copier boshcmd.Copier,
	settingsDir boshdirs.Provider) LogsTarProvider {
	return logsTarProvider{
		compressor:  compressor,
		copier:      copier,
		settingsDir: settingsDir,
	}
}

func (l logsTarProvider) Get(logType string, filters []string) (string, error) {
	var logsDir string

	switch logType {
	case "job":
		if len(filters) == 0 {
			filters = []string{"**/*"}
		}
		logsDir = l.settingsDir.LogsDir()
	case "agent":
		if len(filters) == 0 {
			filters = []string{"**/*"}
		}
		logsDir = l.settingsDir.AgentLogsDir()
	default:
		return "", bosherr.Error("Invalid log type")
	}

	tmpDir, err := l.copier.FilteredCopyToTemp(logsDir, filters)
	if err != nil {
		return "", bosherr.WrapError(err, "Copying filtered files to temp directory")
	}

	defer l.copier.CleanUp(tmpDir)

	tarball, err := l.compressor.CompressFilesInDir(tmpDir)
	if err != nil {
		return "", bosherr.WrapError(err, "Making logs tarball")
	}

	return tarball, nil
}

func (l logsTarProvider) CleanUp(path string) error {
	return l.compressor.CleanUp(path)
}
