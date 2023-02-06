package action

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/agent/logstarprovider"
)

type BundleLogsAction struct {
	logsTarProvider logstarprovider.LogsTarProvider
}

type BundleLogsResponse struct {
	LogsTarPath string `json:"logs_tar_path"`
}

func NewBundleLogs(
	logsTarProvider logstarprovider.LogsTarProvider,
) (action BundleLogsAction) {
	action.logsTarProvider = logsTarProvider
	return
}
func (a BundleLogsAction) IsAsynchronous(_ ProtocolVersion) bool {
	return false
}

func (a BundleLogsAction) IsPersistent() bool {
	return false
}

func (a BundleLogsAction) IsLoggable() bool {
	return true
}

func (a BundleLogsAction) Run(logType string, filters []string) (BundleLogsResponse, error) {
	tarball, err := a.logsTarProvider.Get(logType, filters)
	if err != nil {
		return BundleLogsResponse{}, err
	}

	return BundleLogsResponse{LogsTarPath: tarball}, nil
}

func (a BundleLogsAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a BundleLogsAction) Cancel() error {
	return errors.New("not supported")
}
