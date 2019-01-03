package applier

import (
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
)

type Applier interface {
	Prepare(desiredApplySpec boshas.ApplySpec) error
	ConfigureJobs(desiredApplySpec boshas.ApplySpec) error
	Apply(desiredApplySpec boshas.ApplySpec) error
}
