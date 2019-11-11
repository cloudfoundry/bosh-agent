package action

import (
	"errors"

	blobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
)

type FetchLogsWithSignedURLRequest struct {
	SignedURL        string            `json:"signed_url"`
	LogType          string            `json:"log_type"`
	Filters          []string          `json:"filters"`
	BlobstoreHeaders map[string]string `json:"blobstore_headers"`
}

type FetchLogsWithSignedURLResponse struct {
	SHA1Digest string `json:"sha1"`
}

type FetchLogsWithSignedURLAction struct {
	compressor    boshcmd.Compressor
	copier        boshcmd.Copier
	settingsDir   boshdirs.Provider
	blobDelegator blobdelegator.BlobstoreDelegator
}

func NewFetchLogsWithSignedURLAction(
	compressor boshcmd.Compressor,
	copier boshcmd.Copier,
	settingsDir boshdirs.Provider,
	blobDelegator blobdelegator.BlobstoreDelegator) (action FetchLogsWithSignedURLAction) {
	action.compressor = compressor
	action.copier = copier
	action.settingsDir = settingsDir
	action.blobDelegator = blobDelegator
	return
}

func (a FetchLogsWithSignedURLAction) IsAsynchronous(_ ProtocolVersion) bool {
	return true
}

func (a FetchLogsWithSignedURLAction) IsPersistent() bool {
	return false
}

func (a FetchLogsWithSignedURLAction) IsLoggable() bool {
	return true
}

func (a FetchLogsWithSignedURLAction) Run(request FetchLogsWithSignedURLRequest) (FetchLogsWithSignedURLResponse, error) {
	var logsDir string
	filters := request.Filters

	switch request.LogType {
	case "job":
		if len(request.Filters) == 0 {
			filters = []string{"**/*"}
		}
		logsDir = a.settingsDir.LogsDir()
	case "agent":
		if len(request.Filters) == 0 {
			filters = []string{"**/*"}
		}
		logsDir = a.settingsDir.AgentLogsDir()
	default:
		return FetchLogsWithSignedURLResponse{}, bosherr.Error("Invalid log type")
	}

	tmpDir, err := a.copier.FilteredCopyToTemp(logsDir, filters)
	if err != nil {
		return FetchLogsWithSignedURLResponse{}, bosherr.WrapError(err, "Copying filtered files to temp directory")
	}

	defer a.copier.CleanUp(tmpDir)

	tarball, err := a.compressor.CompressFilesInDir(tmpDir)
	if err != nil {
		return FetchLogsWithSignedURLResponse{}, bosherr.WrapError(err, "Making logs tarball")
	}

	defer func() {
		_ = a.compressor.CleanUp(tarball)
	}()

	_, digest, err := a.blobDelegator.Write(request.SignedURL, tarball, request.BlobstoreHeaders)
	if err != nil {
		return FetchLogsWithSignedURLResponse{}, bosherr.WrapError(err, "Create file on blobstore")
	}

	return FetchLogsWithSignedURLResponse{
		SHA1Digest: digest.String(),
	}, nil
}

func (a FetchLogsWithSignedURLAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a FetchLogsWithSignedURLAction) Cancel() error {
	return errors.New("not supported")
}
