package action

import (
	"errors"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	boshappl "github.com/cloudfoundry/bosh-agent/v2/agent/applier"
	boshas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
)

type PrepareAction struct {
	applier boshappl.Applier
}

func NewPrepare(applier boshappl.Applier) (action PrepareAction) {
	action.applier = applier
	return action
}

func (a PrepareAction) IsAsynchronous(_ ProtocolVersion) bool {
	return true
}

func (a PrepareAction) IsPersistent() bool {
	return false
}

func (a PrepareAction) IsLoggable() bool {
	return true
}

func (a PrepareAction) Run(desiredSpec boshas.V1ApplySpec) (string, error) {
	err := a.applier.Prepare(desiredSpec)
	if err != nil {
		return "", bosherr.WrapError(err, "Preparing apply spec")
	}

	return "prepared", nil
}

func (a PrepareAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a PrepareAction) Cancel() error {
	return errors.New("not supported")
}
