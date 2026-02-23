package action

import (
	"errors"

	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type RemoveDynamicDiskAction struct {
	platform boshplatform.Platform
}

func NewRemoveDynamicDiskAction(platform boshplatform.Platform) RemoveDynamicDiskAction {
	return RemoveDynamicDiskAction{
		platform: platform,
	}
}

func (a RemoveDynamicDiskAction) Run(diskCID string) (interface{}, error) {
	if err := a.platform.CleanupDynamicDisk(diskCID); err != nil {
		return "", bosherr.WrapError(err, "Setting up dynamic disk")
	}

	return map[string]string{}, nil
}

func (a RemoveDynamicDiskAction) IsAsynchronous(_ ProtocolVersion) bool {
	return false
}

func (a RemoveDynamicDiskAction) IsPersistent() bool {
	return false
}

func (a RemoveDynamicDiskAction) IsLoggable() bool {
	return true
}

func (a RemoveDynamicDiskAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a RemoveDynamicDiskAction) Cancel() error {
	return errors.New("not supported")
}
