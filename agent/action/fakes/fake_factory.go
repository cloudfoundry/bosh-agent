package fakes

import (
	"errors"
	"fmt"

	boshaction "github.com/cloudfoundry/bosh-agent/v2/agent/action"
)

type FakeFactory struct {
	registeredActions    map[string]*TestAction
	registeredActionErrs map[string]error
}

func NewFakeFactory() *FakeFactory {
	return &FakeFactory{
		registeredActions:    make(map[string]*TestAction),
		registeredActionErrs: make(map[string]error),
	}
}

func (f *FakeFactory) Create(method string) (boshaction.Action, error) {
	if err := f.registeredActionErrs[method]; err != nil {
		return nil, err
	}
	if action := f.registeredActions[method]; action != nil {
		return action, nil
	}
	return nil, errors.New("Action not found") //nolint:staticcheck
}

func (f *FakeFactory) RegisterAction(method string, action *TestAction) {
	if a := f.registeredActions[method]; a != nil {
		panic(fmt.Sprintf("Action is already registered: %v", a))
	}
	f.registeredActions[method] = action
}

func (f *FakeFactory) RegisterActionErr(method string, err error) {
	if e := f.registeredActionErrs[method]; e != nil {
		panic(fmt.Sprintf("Action err is already registered: %v", e))
	}
	f.registeredActionErrs[method] = err
}

type TestAction struct {
	Asynchronous bool
	Persistent   bool
	Loggable     bool

	ResumeValue interface{}
	ResumeErr   error
	Resumed     bool

	Canceled  bool
	CancelErr error

	ProtocolVersion boshaction.ProtocolVersion
}

func (a *TestAction) IsAsynchronous(protocolVersion boshaction.ProtocolVersion) bool {
	a.ProtocolVersion = protocolVersion
	return a.Asynchronous
}

func (a *TestAction) IsPersistent() bool {
	return a.Persistent
}

func (a *TestAction) IsLoggable() bool {
	return a.Loggable
}

func (a *TestAction) Run(payload []byte) (interface{}, error) {
	return nil, nil
}

func (a *TestAction) Resume() (interface{}, error) {
	a.Resumed = true
	return a.ResumeValue, a.ResumeErr
}

func (a *TestAction) Cancel() error {
	a.Canceled = true
	return a.CancelErr
}
