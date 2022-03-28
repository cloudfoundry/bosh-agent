package action

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
)

type FetchLogsAction struct {
	compressor  boshcmd.Compressor
	copier      boshcmd.Copier
	blobstore   blobstore_delegator.BlobstoreDelegator
	settingsDir boshdirs.Provider
}

func NewFetchLogs(
	compressor boshcmd.Compressor,
	copier boshcmd.Copier,
	blobstore blobstore_delegator.BlobstoreDelegator,
	settingsDir boshdirs.Provider,
) (action FetchLogsAction) {
	action.compressor = compressor
	action.copier = copier
	action.blobstore = blobstore
	action.settingsDir = settingsDir
	return
}

func (a FetchLogsAction) IsAsynchronous(_ ProtocolVersion) bool {
	return true
}

func (a FetchLogsAction) IsPersistent() bool {
	return false
}

func (a FetchLogsAction) IsLoggable() bool {
	return true
}

func (a FetchLogsAction) Run(logType string, filters []string) (map[string]string, error) {
	value := map[string]string{}
	var logsDir string

	switch logType {
	case "job":
		if len(filters) == 0 {
			filters = []string{"**/*"}
		}
		logsDir = a.settingsDir.LogsDir()
	case "agent":
		if len(filters) == 0 {
			filters = []string{"**/*"}
		}
		logsDir = a.settingsDir.AgentLogsDir()
	default:
		return value, bosherr.Error("Invalid log type")
	}

	tmpDir, err := a.copier.FilteredCopyToTemp(logsDir, filters)
	if err != nil {
		return value, bosherr.WrapError(err, "Copying filtered files to temp directory")
	}

	defer a.copier.CleanUp(tmpDir)

	tarball, err := a.compressor.CompressFilesInDir(tmpDir)
	if err != nil {
		return value, bosherr.WrapError(err, "Making logs tarball")
	}

	defer func() {
		_ = a.compressor.CleanUp(tarball)
	}()

	blobID, multidigestSha, err := a.blobstore.Write("", tarball, nil)
	if err != nil {
		return value, bosherr.WrapError(err, "Create file on blobstore")
	}

	value = map[string]string{"blobstore_id": blobID, "sha1": multidigestSha.String()}
	return value, nil
}

func (a FetchLogsAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a FetchLogsAction) Cancel() error {
	return errors.New("not supported")
}
