package action

import "errors"

type ForcefulARPAction struct{}

func NewForcefulARP() ForcefulARPAction {
	return ForcefulARPAction{}
}

func (a ForcefulARPAction) IsAsynchronous() bool {
	return false
}

func (a ForcefulARPAction) IsPersistent() bool {
	return false
}

func (a ForcefulARPAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a ForcefulARPAction) Cancel() error {
	return errors.New("not supported")
}
