package fakes

import (
	"github.com/cloudfoundry/bosh-agent/agent/drain"
)

type FakeScript struct {
	tag        string
	Params     drain.ScriptParams
	ExistsBool bool

	RunCallCount int
	DidRun       bool
	RunError     error
	RunStub      func() error
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
