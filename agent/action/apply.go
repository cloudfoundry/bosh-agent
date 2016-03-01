package action

import (
	"errors"
	"path"

	boshappl "github.com/cloudfoundry/bosh-agent/agent/applier"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type ApplyAction struct {
	applier         boshappl.Applier
	specService     boshas.V1Service
	settingsService boshsettings.Service
	etcDir          string
	fs              boshsys.FileSystem
}

func NewApply(
	applier boshappl.Applier,
	specService boshas.V1Service,
	settingsService boshsettings.Service,
	etcDir string,
	fs boshsys.FileSystem,
) (action ApplyAction) {
	action.applier = applier
	action.specService = specService
	action.settingsService = settingsService
	action.etcDir = etcDir
	action.fs = fs
	return
}

func (a ApplyAction) IsAsynchronous() bool {
	return true
}

func (a ApplyAction) IsPersistent() bool {
	return false
}

func (a ApplyAction) Run(desiredSpec boshas.V1ApplySpec) (string, error) {
	settings := a.settingsService.GetSettings()

	resolvedDesiredSpec, err := a.specService.PopulateDHCPNetworks(desiredSpec, settings)
	if err != nil {
		return "", bosherr.WrapError(err, "Resolving dynamic networks")
	}

	if desiredSpec.ConfigurationHash != "" {
		currentSpec, err := a.specService.Get()
		if err != nil {
			return "", bosherr.WrapError(err, "Getting current spec")
		}

		err = a.applier.Apply(currentSpec, resolvedDesiredSpec)
		if err != nil {
			return "", bosherr.WrapError(err, "Applying")
		}
	}

	err = a.specService.Set(resolvedDesiredSpec)
	if err != nil {
		return "", bosherr.WrapError(err, "Persisting apply spec")
	}

	err = a.writeInstanceData(resolvedDesiredSpec)
	if err != nil {
		return "", err
	}

	return "applied", nil
}

func (a ApplyAction) writeInstanceData(spec boshas.V1ApplySpec) error {
	instanceDir := path.Join(a.etcDir, "instance")
	err := a.fs.WriteFileString(path.Join(instanceDir, "id"), spec.NodeID)
	if err != nil {
		return err
	}
	err = a.fs.WriteFileString(path.Join(instanceDir, "az"), spec.AvailabilityZone)
	if err != nil {
		return err
	}
	err = a.fs.WriteFileString(path.Join(instanceDir, "name"), spec.Deployment)
	if err != nil {
		return err
	}

	return nil
}

func (a ApplyAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a ApplyAction) Cancel() error {
	return errors.New("not supported")
}
