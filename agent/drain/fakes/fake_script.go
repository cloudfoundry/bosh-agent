package fakes

import (
	"github.com/cloudfoundry/bosh-agent/agent/drain"
)

type FakeScript struct {
	Name         string
	ExistsBool   bool
	RunCallCount int
	DidRun       bool
	RunError     error
	RunStub      func(params drain.ScriptParams) error
	RunParams    []drain.ScriptParams
}

func NewFakeScript() *FakeScript {
	return &FakeScript{ExistsBool: true}
}

func (script *FakeScript) Exists() bool {
	return script.ExistsBool
}

func (script *FakeScript) Path() string {
	return "/fake/path"
}

func (script *FakeScript) Run(params drain.ScriptParams) error {
	script.DidRun = true
	script.RunParams = append(script.RunParams, params)
	script.RunCallCount++
	if script.RunStub != nil {
		return script.RunStub(params)
	}
	return script.RunError
}
