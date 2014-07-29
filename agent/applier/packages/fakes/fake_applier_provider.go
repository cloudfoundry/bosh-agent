package fakes

import (
	"github.com/cloudfoundry/bosh-agent/agent/applier/packages"
)

type FakeApplierProvider struct {
	RootApplier         *FakeApplier
	JobSpecificAppliers map[string]*FakeApplier
}

func NewFakeApplierProvider() *FakeApplierProvider {
	return &FakeApplierProvider{
		JobSpecificAppliers: map[string]*FakeApplier{},
	}
}

func (p *FakeApplierProvider) Root() packages.Applier {
	if p.RootApplier == nil {
		panic("Root package applier not found")
	}
	return p.RootApplier
}

func (p *FakeApplierProvider) JobSpecific(jobName string) packages.Applier {
	applier := p.JobSpecificAppliers[jobName]
	if applier == nil {
		panic("Job specific package applier not found")
	}
	return applier
}
