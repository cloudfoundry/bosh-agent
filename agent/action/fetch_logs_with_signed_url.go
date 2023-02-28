package action

import (
	"errors"
	"github.com/cloudfoundry/bosh-agent/agent/logstarprovider"

	blobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type FetchLogsWithSignedURLRequest struct {
	SignedURL string `json:"signed_url"`

	LogType string            `json:"log_type"`
	Filters []string          `json:"filters"`
	Headers map[string]string `json:"headers"`
}

type FetchLogsWithSignedURLResponse struct {
	SHA1Digest string `json:"sha1"`
}

type FetchLogsWithSignedURLAction struct {
	logsTarProvider logstarprovider.LogsTarProvider
	blobDelegator   blobdelegator.BlobstoreDelegator
}

func NewFetchLogsWithSignedURLAction(
	logsTarProvider logstarprovider.LogsTarProvider,
	blobDelegator blobdelegator.BlobstoreDelegator) (action FetchLogsWithSignedURLAction) {
	action.logsTarProvider = logsTarProvider
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
	tarball, err := a.logsTarProvider.Get(request.LogType, request.Filters)
	if err != nil {
		return FetchLogsWithSignedURLResponse{}, err
	}

	defer a.logsTarProvider.CleanUp(tarball)

	_, digest, err := a.blobDelegator.Write(request.SignedURL, tarball, request.Headers)
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
