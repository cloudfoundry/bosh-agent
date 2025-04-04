package logstarprovider

import (
	"runtime"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"

	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
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

func (l logsTarProvider) Get(logTypes string, filters []string) (string, error) {
	var directoriesAndPrefixes []boshcmd.DirToCopy
	var err error

	if len(filters) == 0 {
		filters = []string{"**/*"}
	}

	for _, logType := range strings.Split(logTypes, ",") {
		if logType == "job" {
			directoriesAndPrefixes = append(directoriesAndPrefixes,
				boshcmd.DirToCopy{Dir: l.settingsDir.LogsDir(), Prefix: ""})
			continue
		}
		if logType == "agent" {
			directoriesAndPrefixes = append(directoriesAndPrefixes,
				boshcmd.DirToCopy{Dir: l.settingsDir.AgentLogsDir(), Prefix: ""})
			continue
		}
		if logType == "system" {
			if runtime.GOOS == "linux" {
				directoriesAndPrefixes = append(directoriesAndPrefixes,
					boshcmd.DirToCopy{Dir: "/var/log/", Prefix: "/var/log/"})
			}
			continue
		}
		err = bosherr.Error("Invalid log type")
		return "", err
	}

	tmpDir, err := l.copier.FilteredMultiCopyToTemp(directoriesAndPrefixes, filters)
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
