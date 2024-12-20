package fakes

import (
	boshas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
)

type FakeApplier struct {
	Prepared                bool
	PrepareDesiredApplySpec boshas.ApplySpec
	PrepareError            error

	Applied               bool
	ApplyDesiredApplySpec boshas.ApplySpec
	ApplyError            error

	Configured                 bool
	ConfiguredDesiredApplySpec boshas.ApplySpec
	ConfiguredJobs             []models.Job
	ConfiguredError            error
}

func NewFakeApplier() *FakeApplier {
	return &FakeApplier{}
}

func (s *FakeApplier) Prepare(desiredApplySpec boshas.ApplySpec) error {
	s.Prepared = true
	s.PrepareDesiredApplySpec = desiredApplySpec
	return s.PrepareError
}

func (s *FakeApplier) ConfigureJobs(desiredApplySpec boshas.ApplySpec) error {
	s.Configured = true
	s.ConfiguredDesiredApplySpec = desiredApplySpec
	return s.ConfiguredError
}

func (s *FakeApplier) Apply(desiredApplySpec boshas.ApplySpec) error {
	s.Applied = true
	s.ApplyDesiredApplySpec = desiredApplySpec
	return s.ApplyError
}
