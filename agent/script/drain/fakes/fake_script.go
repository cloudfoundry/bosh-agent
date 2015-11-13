package fakes

import (
	"github.com/cloudfoundry/bosh-agent/agent/script/drain"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type FakeScript struct {
	tag        string
	Params     drain.ScriptParams
	ExistsBool bool

	RunCallCount int
	DidRun       bool
	RunError     error
	RunStub      func() error

	RunAsyncCallCount int
	DidRunAsync       bool
	RunAsyncError     error
	RunAsyncStub      func() error
}

func NewFakeScript(tag string) *FakeScript {
	return &FakeScript{tag: tag, ExistsBool: true}
}

func (s *FakeScript) Tag() string  { return s.tag }
func (s *FakeScript) Path() string { return "/fake/path" }
func (s *FakeScript) Exists() bool { return s.ExistsBool }

func (s *FakeScript) Run() error {
	s.DidRun = true
	s.RunCallCount++
	if s.RunStub != nil {
		return s.RunStub()
	}
	return s.RunError
}

func (s *FakeScript) RunAsync() (boshsys.Process, error) {
	s.DidRunAsync = true
	s.RunAsyncCallCount++
	if s.RunAsyncStub != nil {
		return nil, s.RunAsyncStub()
	}
	return nil, s.RunAsyncError
}
