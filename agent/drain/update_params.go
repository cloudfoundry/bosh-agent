package drain

import (
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
)

type updateParams struct {
	oldSpec boshas.V1ApplySpec
	newSpec boshas.V1ApplySpec
}

func NewUpdateParams(oldSpec, newSpec boshas.V1ApplySpec) ScriptParams {
	return updateParams{
		oldSpec: oldSpec,
		newSpec: newSpec,
	}
}

func (p updateParams) JobChange() string {
	switch {
	case len(p.oldSpec.Jobs()) == 0:
		return "job_new"
	case p.oldSpec.JobSpec.Sha1 == p.newSpec.JobSpec.Sha1:
		return "job_unchanged"
	default:
		return "job_changed"
	}
}

func (p updateParams) HashChange() string {
	switch {
	case p.oldSpec.ConfigurationHash == "":
		return "hash_new"
	case p.oldSpec.ConfigurationHash == p.newSpec.ConfigurationHash:
		return "hash_unchanged"
	default:
		return "hash_changed"
	}
}

func (p updateParams) UpdatedPackages() (pkgs []string) {
	for _, pkg := range p.newSpec.PackageSpecs {
		currentPkg, found := p.oldSpec.PackageSpecs[pkg.Name]
		switch {
		case !found:
			pkgs = append(pkgs, pkg.Name)
		case currentPkg.Sha1 != pkg.Sha1:
			pkgs = append(pkgs, pkg.Name)
		}
	}
	return
}

func (p updateParams) JobState() (string, error) {
	return newPresentedJobState(&p.oldSpec).MarshalToJSONString()
}

func (p updateParams) JobNextState() (string, error) {
	return newPresentedJobState(&p.newSpec).MarshalToJSONString()
}
