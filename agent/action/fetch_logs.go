package action

import (
	"errors"
	"github.com/cloudfoundry/bosh-agent/agent/logstarprovider"

	blobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type FetchLogsAction struct {
	logsTarProvider logstarprovider.LogsTarProvider
	blobstore       blobdelegator.BlobstoreDelegator
}

func NewFetchLogs(
	logsTarProvider logstarprovider.LogsTarProvider,
	blobstore blobdelegator.BlobstoreDelegator,
) (action FetchLogsAction) {
	action.logsTarProvider = logsTarProvider
	action.blobstore = blobstore
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

func (a FetchLogsAction) Run(logType string, filters []string) (value map[string]string, err error) {
	tarball, err := a.logsTarProvider.Get(logType, filters)
	if err != nil {
		return
	}

	defer a.logsTarProvider.CleanUp(tarball)

	blobID, multidigestSha, err := a.blobstore.Write("", tarball, nil)
	if err != nil {
		err = bosherr.WrapError(err, "Create file on blobstore")
		return
	}

	value = map[string]string{"blobstore_id": blobID, "sha1": multidigestSha.String()}
	return
}

func (a FetchLogsAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a FetchLogsAction) Cancel() error {
	return errors.New("not supported")
}
