package fakes

import (
	boshinf "github.com/cloudfoundry/bosh-agent/infrastructure"
)

type FakeRegistryProvider struct {
	GetRegistryRegistry boshinf.Registry
}

func (p *FakeRegistryProvider) GetRegistry() boshinf.Registry {
	return p.GetRegistryRegistry
}
