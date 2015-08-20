package fakes

import (
	"github.com/cloudfoundry/bosh-agent/agent/scriptrunner"
)

type FakeScriptProvider struct {
	Script *FakeScript
}

func NewFakeScriptProvider() (provider *FakeScriptProvider) {
	provider = &FakeScriptProvider{}
	provider.Script = NewFakeScript()
	return
}

func (p *FakeScriptProvider) Get(scriptPath string) (script scriptrunner.Script) {
	script = p.Script
	return
}
