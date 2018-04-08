package fakes

import (
	"sync"

	models "github.com/cloudfoundry/bosh-agent/agent/applier/models"
)

type FakeApplier struct {
	ActionsCalled []string

	PreparedPackages []models.Package
	PrepareError     error

	AppliedPackages []models.Package
	ApplyError      error

	KeptOnlyPackages []models.Package
	KeepOnlyErr      error
	applyMutex       sync.Mutex
	PrepareStub      func(pkg models.Package) error
}

func NewFakeApplier() *FakeApplier {
	return &FakeApplier{
		AppliedPackages: []models.Package{},
	}
}

func (s *FakeApplier) Prepare(pkg models.Package) error {
	s.applyMutex.Lock()
	s.ActionsCalled = append(s.ActionsCalled, "Prepare")
	s.PreparedPackages = append(s.PreparedPackages, pkg)
	s.applyMutex.Unlock()
	if s.PrepareStub != nil {
		return s.PrepareStub(pkg)
	}
	return s.PrepareError
}

func (s *FakeApplier) Apply(pkg models.Package) error {
	s.ActionsCalled = append(s.ActionsCalled, "Apply")
	s.AppliedPackages = append(s.AppliedPackages, pkg)
	return s.ApplyError
}

func (s *FakeApplier) KeepOnly(pkgs []models.Package) error {
	s.ActionsCalled = append(s.ActionsCalled, "KeepOnly")
	s.KeptOnlyPackages = pkgs
	return s.KeepOnlyErr
}
