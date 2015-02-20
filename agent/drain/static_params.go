package drain

import (
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
)

type staticParams struct {
	jobChange       string
	hashChange      string
	updatedPackages []string

	oldSpec boshas.V1ApplySpec
	newSpec *boshas.V1ApplySpec
}

func NewShutdownParams(
	oldSpec boshas.V1ApplySpec,
	newSpec *boshas.V1ApplySpec,
) ScriptParams {
	return staticParams{
		jobChange:       "job_shutdown",
		hashChange:      "hash_unchanged",
		updatedPackages: []string{},
		oldSpec:         oldSpec,
		newSpec:         newSpec,
	}
}

func NewStatusParams(
	oldSpec boshas.V1ApplySpec,
	newSpec *boshas.V1ApplySpec,
) ScriptParams {
	return staticParams{
		jobChange:       "job_check_status",
		hashChange:      "hash_unchanged",
		updatedPackages: []string{},
		oldSpec:         oldSpec,
		newSpec:         newSpec,
	}
}

func (p staticParams) JobChange() (change string)       { return p.jobChange }
func (p staticParams) HashChange() (change string)      { return p.hashChange }
func (p staticParams) UpdatedPackages() (pkgs []string) { return p.updatedPackages }

func (p staticParams) JobState() (string, error) {
	return newPresentedJobState(&p.oldSpec).MarshalToJSONString()
}

func (p staticParams) JobNextState() (string, error) {
	return newPresentedJobState(p.newSpec).MarshalToJSONString()
}
