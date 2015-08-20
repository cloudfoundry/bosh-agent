package fakes

type FakeScript struct {
	ExistsBool    bool
	DidRun        bool
	RunExitStatus int
	RunError      error
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

func (script *FakeScript) Run() (value int, err error) {
	script.DidRun = true
	value = script.RunExitStatus
	err = script.RunError
	return
}
