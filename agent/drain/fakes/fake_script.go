package fakes

import (
	"github.com/cloudfoundry/bosh-agent/agent/drain"
)

type FakeScript struct {
	ExistsBool    bool
	DidRun        bool
	RunExitStatus int
	RunError      error
	RunParams     drain.ScriptParams
}

func NewFakeScript() (script *FakeScript) {
	script = &FakeScript{
		RunExitStatus: 1,
	}
	return
}

func (script *FakeScript) Exists() bool {
	return script.ExistsBool
}

func (script *FakeScript) Path() string {
	return "/fake/path"
}

func (script *FakeScript) Run(params drain.ScriptParams) (value int, err error) {
	script.DidRun = true
	script.RunParams = params
	value = script.RunExitStatus
	err = script.RunError
	return
}
