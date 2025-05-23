package action

import (
	"errors"
	"os"
	"path"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	boshappl "github.com/cloudfoundry/bosh-agent/v2/agent/applier"
	boshas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"

	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

const (
	userBaseDirPermissions      = os.FileMode(0755)
	userInstanceFilePermissions = os.FileMode(0644)
)

type ApplyAction struct {
	applier         boshappl.Applier
	specService     boshas.V1Service
	settingsService boshsettings.Service
	instanceDir     string
	fs              boshsys.FileSystem
}

func NewApply(
	applier boshappl.Applier,
	specService boshas.V1Service,
	settingsService boshsettings.Service,
	dirProvider directories.Provider,
	fs boshsys.FileSystem,
) (action ApplyAction) {
	action.applier = applier
	action.specService = specService
	action.settingsService = settingsService
	action.instanceDir = dirProvider.InstanceDir()
	action.fs = fs
	return
}

func (a ApplyAction) IsAsynchronous(_ ProtocolVersion) bool {
	return true
}

func (a ApplyAction) IsPersistent() bool {
	return false
}

func (a ApplyAction) IsLoggable() bool {
	return true
}

func (a ApplyAction) Run(desiredSpec boshas.V1ApplySpec) (string, error) {
	settings := a.settingsService.GetSettings()

	resolvedDesiredSpec, err := a.specService.PopulateDHCPNetworks(desiredSpec, settings)
	if err != nil {
		return "", bosherr.WrapError(err, "Resolving dynamic networks")
	}

	if desiredSpec.ConfigurationHash != "" {
		err = a.applier.Apply(resolvedDesiredSpec)
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
	err := a.writeInstanceField("id", spec.NodeID)
	if err != nil {
		return err
	}
	err = a.writeInstanceField("az", spec.AvailabilityZone)
	if err != nil {
		return err
	}
	err = a.writeInstanceField("name", spec.Name)
	if err != nil {
		return err
	}
	err = a.writeInstanceField("deployment", spec.Deployment)
	if err != nil {
		return err
	}

	err = a.fs.Chmod(a.instanceDir, userBaseDirPermissions)
	if err != nil {
		return err
	}

	return nil
}

func (a ApplyAction) writeInstanceField(filename string, instanceField string) error {
	instanceFieldFilePath := path.Join(a.instanceDir, filename)
	err := a.fs.WriteFileString(instanceFieldFilePath, instanceField)
	if err != nil {
		return err
	}

	err = a.fs.Chmod(instanceFieldFilePath, userInstanceFilePermissions)
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
