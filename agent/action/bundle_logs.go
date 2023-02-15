package action

import (
	"errors"
	"github.com/cloudfoundry/bosh-agent/agent/logstarprovider"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type BundleLogsAction struct {
	logsTarProvider logstarprovider.LogsTarProvider
	fs              boshsys.FileSystem
}

type BundleLogsRequest struct {
	OwningUser string `json:"owning_user"`

	LogType string   `json:"log_type"`
	Filters []string `json:"filters"`
}

type BundleLogsResponse struct {
	LogsTarPath string `json:"logs_tar_path"`
}

func NewBundleLogs(
	logsTarProvider logstarprovider.LogsTarProvider,
	fs boshsys.FileSystem,

) (action BundleLogsAction) {
	action.logsTarProvider = logsTarProvider
	action.fs = fs
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

func (a BundleLogsAction) Run(request BundleLogsRequest) (BundleLogsResponse, error) {
	tarball, err := a.logsTarProvider.Get(request.LogType, request.Filters)
	if err != nil {
		return BundleLogsResponse{}, err
	}

	if request.OwningUser != "" {
		err = a.fs.Chown(tarball, request.OwningUser)
		if err != nil {
			return BundleLogsResponse{}, err
		}
	}

	return BundleLogsResponse{LogsTarPath: tarball}, nil
}

func (a BundleLogsAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a BundleLogsAction) Cancel() error {
	return errors.New("not supported")
}
