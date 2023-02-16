package action

import (
	"errors"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type RemoveFileAction struct {
	fs boshsys.FileSystem
}

func NewRemoveFile(
	fs boshsys.FileSystem) RemoveFileAction {
	return RemoveFileAction{
		fs: fs,
	}
}
func (r RemoveFileAction) IsAsynchronous(_ ProtocolVersion) bool {
	return false
}

func (r RemoveFileAction) IsPersistent() bool {
	return false
}

func (r RemoveFileAction) IsLoggable() bool {
	return true
}

func (r RemoveFileAction) Run(path string) (string, error) {
	return path, r.fs.RemoveAll(path)
}

func (r RemoveFileAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (r RemoveFileAction) Cancel() error {
	return errors.New("not supported")
}
