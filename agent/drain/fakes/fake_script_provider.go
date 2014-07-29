package fakes

import (
	boshdrain "github.com/cloudfoundry/bosh-agent/agent/drain"
)

type FakeScriptProvider struct {
	NewScriptTemplateName string
	NewScriptScript       *FakeScript
}

func NewFakeScriptProvider() (provider *FakeScriptProvider) {
	provider = &FakeScriptProvider{}
	provider.NewScriptScript = NewFakeScript()
	return
}

func (p *FakeScriptProvider) NewScript(templateName string) (drainScript boshdrain.Script) {
	p.NewScriptTemplateName = templateName
	drainScript = p.NewScriptScript
	return
}
