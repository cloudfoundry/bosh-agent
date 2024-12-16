package action

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/v2/platform"
)

type ShutdownAction struct {
	platform platform.Platform
}

func NewShutdown(platform platform.Platform) ShutdownAction {
	return ShutdownAction{
		platform: platform,
	}
}

func (a ShutdownAction) IsAsynchronous(_ ProtocolVersion) bool {
	return false
}

func (a ShutdownAction) IsPersistent() bool {
	return false
}

func (a ShutdownAction) IsLoggable() bool {
	return true
}

func (a ShutdownAction) Run() (string, error) {
	err := a.platform.Shutdown()
	return "", err
}

func (a ShutdownAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a ShutdownAction) Cancel() error {
	return errors.New("not supported")
}
